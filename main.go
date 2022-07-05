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
	"io/ioutil"
	"net/http"
	"time"

	"database/sql"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/logger"
)

/*
type program struct{}

func (p *program) Start(s service.Service) error {
	err := initdb()
	if err != nil {
		return err
	}
	go launchapi()
	go p.Run()
	return nil
}

func (p *program) Run() error {
	mainLoop()
	return nil
}

func (p *program) Stop(s service.Service) error {
	db.Close()
	return nil
}
*/
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
}

type JobState int

const (
	Submitted JobState = iota
	ExaminingSource
	BuildVideoFilter
	BuildAudioFilter
	Transcoding
	Complete
	Failed
)

var (
	// databasefile = "//citadel.somuchcrypto.com/media/other/transcode-factory.db"
	databasefile = "f:/transcode-factory.db"
	db           *sql.DB
)

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
    autocrop INTEGER
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
    `); err != nil {
		return err
	}
	return nil
}

func mainLoop() {
	for {
		tj, err := pullNextJob()
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
				logger.Fatal("failed to cleanup job: %q", err)
			}
			continue
		}

		logger.Infof("job id %d: determining length in frames", tj.Id)
		if err := updateSourceMetadata(&tj); err != nil {
			logger.Errorf("failed to determine number of frames in file: %v", err)
			tj.State = Failed
			if err := finishJob(&tj, nil); err != nil {
				logger.Fatal("failed to cleanup job: %q", err)
			}
			continue
		}

		logger.Infof("job id %d: building video filter graph", tj.Id)
		updateJobStatus(tj.Id, BuildVideoFilter)
		if tj.JobDefinition.Autocrop {
			if err := addCrop(&tj); err != nil {
				logger.Errorf("updatefilters failed on job %d: %q", tj.Id, err)
				tj.State = Failed
				if err := finishJob(&tj, nil); err != nil {
					logger.Fatal("failed to cleanup job: %q", err)
				}
				continue
			}
		} else {
			tx, err := db.Begin()
			if err != nil {
				logger.Errorf("failed to begin transaction: %q", err)
				tj.State = Failed
				if err := finishJob(&tj, nil); err != nil {
					logger.Fatal("failed to cleanup job: %q", err)
				}
				continue
			}
			_, err = tx.Exec("UPDATE active_job SET vfilter = (SELECT video_filters FROM transcode_queue WHERE id =?)", tj.Id)
			if err != nil {
				logger.Errorf("failed to copy video filters to active job: %q, rollback: %q", err, tx.Rollback())
				tj.State = Failed
				if err := finishJob(&tj, nil); err != nil {
					logger.Fatal("failed to cleanup job: %q", err)
				}
				continue
			}
			if err = tx.Commit(); err != nil {
				logger.Errorf("failed to commit transaction: %q", err)
				tx.Rollback()
				tj.State = Failed
				if err := finishJob(&tj, nil); err != nil {
					logger.Fatal("failed to cleanup job: %q", err)
				}
				continue
			}
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

func launchapi() {
	http.HandleFunc("/statusz", display_rows)
	http.HandleFunc("/add", newtranscode)
	go http.ListenAndServe(":51218", nil)
}

func main() {
	logger.Init("transcode-factory", true, true, ioutil.Discard)
	var err error
	db, err = sql.Open("sqlite", databasefile)
	if err != nil {
		logger.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	if err := initdb(); err != nil {
		logger.Fatalf("failed to prepare database: %v", err)
	}

	launchapi()
	mainLoop()
	/*
		svConfig := &service.Config{
			Name:        "Transcode-Factory",
			DisplayName: "Transcode-Factory",
			Description: "Service to automatically invoke ffmpeg.",
		}
		prg := &program{}
		s, err := service.New(prg, svConfig)
		if err != nil {
			logger.Fatal(err)
		}
		if err := s.Run(); err != nil {
			logger.Fatalf("%q", err)
		}
		return
	*/
}
