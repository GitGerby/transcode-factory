// Copyright 2022 GearnsC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"database/sql"

	"github.com/gitgerby/transcode-factory/internal/pkg/config"
	"github.com/gitgerby/transcode-factory/internal/pkg/ffwrap"
	"github.com/gitgerby/transcode-factory/internal/pkg/priority"

	"github.com/google/logger"
	"github.com/kardianos/service"

	"golang.org/x/sync/errgroup"

	_ "modernc.org/sqlite"
)

type program struct{}

type JobState string

const (
	JOB_SUBMITTED        = "submitted"
	JOB_METADATA         = "probing source metadata"
	JOB_BUILDVIDEOFILTER = "constructing video filter graph"
	JOB_BUILDAUDIOFILTER = "constructing audio filter graph"
	JOB_PENDINGTRANSCODE = "waiting for transcoder slot"
	JOB_TRANSCODING      = "copying or transcoding media"
	JOB_SUCCESS          = "completed successfully"
	JOB_FAILED           = "job failed"
	JOB_CANCELLED        = "job cancelled before completion"
)

type TranscodeJob struct {
	Id            int
	JobDefinition ffwrap.TranscodeRequest
	SourceMeta    ffwrap.MediaMetadata
	State         JobState
}

var (
	databasefile string
	db           *sql.DB
	ctx          context.Context
	cancelCtx    func()
	encodeLogDir string
	wsHub        *Hub
	tfConfig     config.TFConfig
)

func (p *program) Start(s service.Service) error {
	go p.Run()
	return nil
}

func (p *program) Run() {
	var err error
	// Lower process priority to reduce impact on interactive uses of the host
	err = priority.LowerPriority()
	if err != nil {
		logger.Errorf("lowerPriority() returned: %v", err)
	}

	// Create context and setup for service stop
	ctx, cancelCtx = context.WithCancel(context.Background())

	// Find and connect to the database
	databasefile = fmt.Sprintf("%s?_pragma=busy_timeout(5000)", *tfConfig.DBPath)

	if err := os.MkdirAll(filepath.Dir(databasefile), 0644); err != nil {
		logger.Fatalf("failed to create directory for database file: %v", err)
	}
	db, err = sql.Open("sqlite", databasefile)

	if err != nil {
		logger.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	if err := initDbTables(db); err != nil {
		logger.Fatalf("failed to prepare database: %v", err)
	}

	ffwrap.SetBinaryLocations(*tfConfig.FfmpegPath, *tfConfig.FfprobePath)

	// Create log directory if it does not exist
	encodeLogDir = *tfConfig.LogDirectory
	if err := os.MkdirAll(encodeLogDir, 0644); err != nil {
		logger.Errorf("failed to create transcode log directory: %v", err)
	}

	// Begin execution
	wsHub = newHub()
	launchApi()
	go cropManager()
	go copyManager()
	go wsHub.run()
	go wsHub.feedSockets()
	mainLoop()
}

func (p *program) Stop(s service.Service) error {
	logger.Info("Service received stop request")
	cancelCtx()
	// Give time for the goroutines to finish up
	time.Sleep(time.Second * 1)
	db.Close()
	return nil
}

func launchApi() {
	http.HandleFunc("/statusz", func(w http.ResponseWriter, r *http.Request) {
		statuszHandler(w, statuszTemplate)
	})
	http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		addHandler(w, r, wsHub.refresh)
	})
	http.HandleFunc("/bulkadd", func(w http.ResponseWriter, r *http.Request) {
		bulkAddHandler(w, r, wsHub.refresh)
	})
	http.HandleFunc("/logstream", logStream)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/statusz", http.StatusFound)
	})
	go http.ListenAndServe(fmt.Sprintf("%s:%d", *tfConfig.ListenAddress, *tfConfig.ListenPort), nil)
}

// initDbTables sets up the database schema by creating tables if they do not exist.
// We drop the active_jobs table to remove lingering artifacts from unclean shutdowns.
func initDbTables(db *sql.DB) error {
	if _, err := db.Exec(`
  CREATE TABLE IF NOT EXISTS transcode_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT,
    destination TEXT,
    crf INTEGER,
    srt_files BLOB,
	  codec TEXT,
    video_filters TEXT,
    audio_filters TEXT,
    autocrop INTEGER,
		crop_complete INTEGER DEFAULT 0
  );

	DROP TABLE IF EXISTS active_job;
  DROP TABLE IF EXISTS active_jobs;
  CREATE TABLE IF NOT EXISTS active_jobs (
    id INTEGER PRIMARY KEY,
    job_state TEXT,
		FOREIGN KEY (id) REFERENCES transcode_queue (id)
  );

  CREATE TABLE IF NOT EXISTS completed_jobs (
    id INTEGER PRIMARY KEY,
    source TEXT,
    destination TEXT,
    autocrop INTEGER,
    ffmpegargs TEXT,
		status INTEGER
	);

  CREATE TABLE IF NOT EXISTS source_metadata (
		id INTEGER PRIMARY KEY,
		codec TEXT,
		width INTEGER,
		height INTEGER,
		duration TEXT,
	FOREIGN KEY (id) REFERENCES transcode_queue (id)
	);

  CREATE TABLE IF NOT EXISTS log_files (
		id INTEGER PRIMARY KEY,
		logfile TEXT,
		FOREIGN KEY (id) REFERENCES active_jobs (id)
	);
    `); err != nil {
		return err
	}
	return nil
}

func mainLoop() {
	tg := new(errgroup.Group)

	tg.SetLimit(*tfConfig.TranscodeLimit)

	for {
		// pull the next available job
		tj, err := pullNextTranscode()
		if err == sql.ErrNoRows {
			time.Sleep(2 * time.Second)
			continue
		} else if err != nil {
			logger.Fatalf("failed to pull next work item: %q", err)
		}

		logger.Infof("job id %d: beginning processing", tj.Id)
		if err := updateJobStatus(tj.Id, JOB_METADATA); err != nil {
			logger.Errorf("failed to mark job active: %v", err)
			tj.State = JOB_FAILED
			if err := finishJob(&tj, nil); err != nil {
				logger.Fatalf("failed to cleanup job: %q", err)
			}
			continue
		}

		logger.Infof("job id %d: determining source metadata", tj.Id)
		if err := updateSourceMetadata(&tj); err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Warningf("service shutting down: %v", err)
				return
			}
			logger.Errorf("ffprobe failed: %v", err)
			tj.State = JOB_FAILED
			if err := finishJob(&tj, nil); err != nil {
				logger.Fatalf("failed to cleanup job: %q", err)
			}
			continue
		}

		// remove the job from the active list until a thread is launched to run it
		if err := deactivateJob(tj.Id); err != nil {
			logger.Errorf("deactivateJob(%d): %v", tj.Id, err)
		}

		tg.Go(func() error {
			// Mark job active
			logger.Infof("job id %d: beginning transcode", tj.Id)
			err := updateJobStatus(tj.Id, JOB_TRANSCODING)
			if err != nil {
				logger.Errorf("failed to update job status: %v", err)
			}

			var args []string
			args, err = transcodeMedia(&tj)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					logger.Errorf("service shutting down: %v", err)
					return err
				}
				logger.Errorf("transcodeMedia() error: %q", err)
				tj.State = JOB_FAILED
				if err := finishJob(&tj, nil); err != nil {
					logger.Fatalf("failed to cleanup job: %q", err)
				}
			}
			updateJobStatus(tj.Id, JOB_SUCCESS)
			tj.State = JOB_SUCCESS
			finishJob(&tj, args)
			logger.Infof("job id %d: complete", tj.Id)
			return nil
		})
		// Give time for the job to start
		time.Sleep(100 * time.Millisecond)
	}
}

func cropManager() {
	cg := new(errgroup.Group)

	cg.SetLimit(*tfConfig.CropLimit)
	logger.Infof("crop detect thread listening; limit %v simultaneous jobs", *tfConfig.CropLimit)

	for {
		tj, err := pullNextCrop()
		if err == sql.ErrNoRows {
			time.Sleep(2 * time.Second)
			continue
		} else if err != nil {
			logger.Fatalf("failed to pull next autocrop item: %q", err)
		}

		cg.Go(func() error {
			logger.Infof("job id %d: building video filter graph", tj.Id)
			updateJobStatus(tj.Id, JOB_BUILDVIDEOFILTER)
			err := updateSourceMetadata(&tj)
			if errors.Is(err, context.Canceled) {
				return err
			} else if err != nil {
				logger.Errorf("job id %d: failed to determine source metadata: %q", tj.Id, err)
			}

			err = compileVF(&tj)
			if err != nil {
				logger.Errorf("job id %d: failed to compile vf: %q", tj.Id, err)
			}
			updateJobStatus(tj.Id, JOB_PENDINGTRANSCODE)
			err = deactivateJob(tj.Id)
			if err != nil {
				logger.Errorf("job id %d: failed to deactivate job: %q", tj.Id, err)
			}
			return nil
		})
		time.Sleep(100 * time.Millisecond) // give time for the job to activate
	}
}

func copyManager() {
	cwg := new(errgroup.Group)

	cwg.SetLimit(*tfConfig.CopyLimit)
	logger.Infof("copy manager waiting, max %d simultaneous jobs", *tfConfig.CopyLimit)

	for {
		tj, err := pullNextCopy()
		if err == sql.ErrNoRows {
			time.Sleep(250 * time.Millisecond)
			continue
		} else if err != nil {
			logger.Errorf("failed to pull next copy item: %q", err)
		}

		cwg.Go(func() error {
			if err := updateSourceMetadata(&tj); err != nil {
				logger.Errorf("failed to update job %d metadata: %q", tj.Id, err)
				return nil
			}
			logger.Infof("starting copy for %#v", tj)
			if err := updateJobStatus(tj.Id, JOB_PENDINGTRANSCODE); err != nil {
				logger.Errorf("failed to update job %d status: %q", tj.Id, err)
				return nil
			}
			if err := createDestinationParent(tj.JobDefinition.Destination); err != nil {
				logger.Errorf("failed to create destination directory: %v", err)
				return nil
			}
			if err := updateSourceMetadata(&tj); err != nil {
				logger.Errorf("failed to update source metadata for job %d: %v", tj.Id, err)
			}
			if err := updateJobStatus(tj.Id, JOB_TRANSCODING); err != nil {
				logger.Errorf("failed to update job: %d with error: %v", tj.Id, err)
			}

			args, err := ffwrap.FfmpegTranscode(ctx, tj.JobDefinition)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				logger.Errorf("job id %d: failed to run ffmpeg copy with err: %v", tj.Id, err)
				if err := finishJob(&tj, []string{}); err != nil {
					logger.Errorf("failed to cleanup job: %q", err)
				}
			}
			tj.State = JOB_SUCCESS
			finishJob(&tj, args)
			logger.Infof("job id %d: complete", tj.Id)
			return nil
		})
	}
}

func main() {
	logger.Init("transcode-factory", true, true, io.Discard)

	svcFlag := flag.String("service", "", "Control the system service.")
	configPath := flag.String("config", config.DefaultConfigPath, "Path to the config file.")

	flag.Parse()

	// test the config file specified by the configPath flag if it doesn't exist
	// generate a default config and write it to the location specified
	if _, err := os.Stat(*configPath); errors.Is(err, os.ErrNotExist) {
		err := config.DefaultConfiguration().WriteConfig(*configPath)
		if err != nil {
			logger.Fatalf("failed to create default config: %q", err)
		}
	} else if err != nil {
		logger.Fatalf("failed to stat config file: %q", err)
	}

	// load the config file
	err := tfConfig.Parse(*configPath)
	if err != nil {
		logger.Fatalf("failed to parse config: %q", err)
	}

	svcConfig := &service.Config{
		Name:        "TranscodeFactory",
		DisplayName: "Transcode factory service",
		Description: "Service that listens for transcode jobs and acts on them.",
	}

	prg := &program{}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		logger.Fatalf("service.New failed: %q", err)
	}

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			logger.Errorf("valid service actions: %q", service.ControlAction)
			logger.Fatalf("failed to execute service action: %q", err)
		}
		return
	}

	s.Run()
}
