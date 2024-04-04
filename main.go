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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"database/sql"

	"github.com/google/logger"
	"github.com/kardianos/service"
	"golang.org/x/sync/errgroup"
	_ "modernc.org/sqlite"
)

type program struct{}

type TranscodeRequest struct {
	Source        string   `json:"source"`
	Destination   string   `json:"destination"`
	Srt_files     []string `json:"srt_files"`
	Crf           int      `json:"crf"`
	Autocrop      bool     `json:"autocrop"`
	Video_filters string   `json:"video_filters"`
	Audio_filters string   `json:"audio_filters"`
	Codec         string   `json:"codec"`
	CurrentFrame  int
}

type TranscodeJob struct {
	Id            int
	JobDefinition TranscodeRequest
	SourceMeta    MediaMetadata
	State         JobState
	CurrentFrame  int
}

type MediaMetadata struct {
	TotalFrames int
	Codec       string
	Width       int
	Height      int
}

type JobState int

const (
	Submitted JobState = iota
	ExaminingSource
	BuildVideoFilter
	BuildAudioFilter
	AwaitingTranscode
	Transcoding
	Complete
	Failed
	Cancelled
)

var (
	databasefile       string
	db                 *sql.DB
	ctx                context.Context
	stop_ctx           func()
	ffmpegbinary       string
	ffprobebinary      string
	transcode_log_path string
	muCopy             sync.Mutex
	queueCopy          []*TranscodeJob
)

func (p *program) Start(s service.Service) error {
	go p.Run()
	return nil
}

func (p *program) Run() {
	var err error
	// Create context and setup for service stop
	ctx, stop_ctx = context.WithCancel(context.Background())

	// Find and connect to the database
	dbpenv := os.Getenv("TF_DB_PATH")
	if dbpenv != "" {
		databasefile = fmt.Sprintf("%s?_pragma=busy_timeout(5000)", dbpenv)
	} else {
		databasefile = filepath.Join(os.Getenv("PROGRAMDATA"), "TranscodeFactory", "transcodefactory.db?_pragma=busy_timeout(5000)")
	}
	if err := os.MkdirAll(filepath.Dir(databasefile), 0644); err != nil {
		logger.Fatalf("failed to create directory for database file: %v", err)
	}
	db, err = sql.Open("sqlite", databasefile)

	if err != nil {
		logger.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	if err := initdb(); err != nil {
		logger.Fatalf("failed to prepare database: %v", err)
	}

	// Find ffmpeg binary to use
	ffmenv := os.Getenv("TF_FFMPEG")
	if ffmenv != "" {
		ffmpegbinary = ffmenv
	} else {
		// default to using ffmpeg from PATH
		ffmpegbinary = "ffmpeg.exe"
	}

	// Find ffprobe binary to use
	ffpenv := os.Getenv("TF_FFPROBE")
	if ffpenv != "" {
		ffprobebinary = ffpenv
	} else {
		// default to using ffmpeg from PATH
		ffprobebinary = "ffprobe.exe"
	}

	tflpenv := os.Getenv("TF_LOG_PATH")
	if tflpenv != "" {
		transcode_log_path = tflpenv
	} else {
		transcode_log_path = filepath.Join(os.Getenv("PROGRAMDATA"), "TranscodeFactory", "encoder_logs")
	}
	if err := os.MkdirAll(transcode_log_path, 0644); err != nil {
		logger.Errorf("failed to create transcode log directory: %v", err)
	}

	// Begin execution
	launchApi()
	go cropManager()
	go copyManager()
	mainLoop()
}

func (p *program) Stop(s service.Service) error {
	logger.Info("Service received stop request")
	stop_ctx()
	db.Close()
	return nil
}

func initdb() error {
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
  CREATE TABLE IF NOT EXISTS active_job (
    ffmpeg_pid INTEGER,
    job_state INTEGER,
    current_frame INTEGER,
    total_frames INTEGER,
    vfilter TEXT,
    afilter TEXT,
    source_codec TEXT,
		speed TEXT,
    id INTEGER PRIMARY KEY,
    FOREIGN KEY (id)
      REFERENCES transcode_queue (id)
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
		FOREIGN KEY (id)
			REFERENCES transcode_queue (id)
	);
    `); err != nil {
		return err
	}
	return nil
}

func mainLoop() {
	for {
		tj, err := pullNextTranscode()
		if err == sql.ErrNoRows {
			time.Sleep(10 * time.Second)
			continue
		} else if err != nil {
			logger.Fatalf("failed to pull next work item: %q", err)
		}

		logger.Infof("job id %d: beginning processing", tj.Id)
		if err := updateJobStatus(tj.Id, ExaminingSource); err != nil {
			logger.Errorf("failed to mark job active: %v", err)
			tj.State = Failed
			if err := finishJob(&tj, nil); err != nil {
				logger.Fatalf("failed to cleanup job: %q", err)
			}
			continue
		}

		logger.Infof("job id %d: determining source metadata", tj.Id)
		if err := updateSourceMetadata(&tj); err != nil {
			logger.Errorf("ffprobe failed: %v", err)
			tj.State = Failed
			if err := finishJob(&tj, nil); err != nil {
				logger.Fatalf("failed to cleanup job: %q", err)
			}
			continue
		}

		logger.Infof("job id %d: beginning transcode", tj.Id)
		updateJobStatus(tj.Id, Transcoding)

		args, err := transcodeMedia(&tj)
		if err != nil {
			logger.Errorf("transcodeMedia() error: %q", err)
			tj.State = Failed
			if err := finishJob(&tj, nil); err != nil {
				logger.Fatalf("failed to cleanup job: %q", err)
			}
			continue
		}

		updateJobStatus(tj.Id, Complete)
		tj.State = Complete
		finishJob(&tj, args)
		logger.Infof("job id %d: complete", tj.Id)
	}
}

func launchApi() {
	http.HandleFunc("/statusz", display_rows)
	http.HandleFunc("/add", newtranscode)
	go http.ListenAndServe(":51218", nil)
}

func cropManager() {
	cg := new(errgroup.Group)
	cg.SetLimit(2)
	logger.Infof("crop detect thread listening")
	for {
		tj, err := pullNextCrop()
		if err == sql.ErrNoRows {
			time.Sleep(10 * time.Second)
			continue
		} else if err != nil {
			logger.Fatalf("failed to pull next autocrop item: %q", err)
		}

		logger.Infof("job id %d: building video filter graph", tj.Id)
		updateJobStatus(tj.Id, BuildVideoFilter)

		cg.Go(func() error {
			err := updateSourceMetadata(&tj)
			if err != nil {
				logger.Errorf("job id %d: failed to determine source metadata: %q", tj.Id, err)
			}

			err = compileVF(&tj)
			if err != nil {
				logger.Errorf("job id %d: failed to compile vf: %q", tj.Id, err)
			}
			updateJobStatus(tj.Id, AwaitingTranscode)
			err = deactivateJob(tj.Id)
			if err != nil {
				logger.Errorf("job id %d: failed to deactivate job: %q", tj.Id, err)
			}
			return nil
		})
	}
}

func copyManager() {
	cwg := new(errgroup.Group)
	// don't run more than 2 copy threads at a time.
	cwg.SetLimit(2)
	logger.Info("copy manager waiting")

	for {
		if len(queueCopy) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		tj := dequeueCopy()
		cwg.Go(func() error {
			updateJobStatus(tj.Id, Transcoding)
			args, err := ffmpegTranscode(*tj)
			if err != nil {
				logger.Errorf("job id %d: failed to run ffmpeg copy with args: %v", tj.Id, args)
				if err := finishJob(tj, nil); err != nil {
					logger.Fatalf("failed to cleanup job: %q", err)
				}
			}
			updateJobStatus(tj.Id, Complete)
			tj.State = Complete
			finishJob(tj, args)
			logger.Infof("job id %d: complete", tj.Id)
			return nil
		})
	}
}

func dequeueCopy() *TranscodeJob {
	muCopy.Lock()
	defer func() { muCopy.Unlock() }()
	nextCopy := queueCopy[0]
	queueCopy = queueCopy[1:]
	return nextCopy
}

func enqueueCopy(tj *TranscodeJob) {
	muCopy.Lock()
	defer func() { muCopy.Unlock() }()
	queueCopy = append(queueCopy, tj)
}

func main() {
	logger.Init("transcode-factory", true, true, io.Discard)

	svcFlag := flag.String("service", "", "Control the system service.")
	flag.Parse()

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
