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
	"flag"
	"io"
	"net/http"
	"time"

	"database/sql"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/logger"
	"github.com/kardianos/service"
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
	databasefile = "f:/transcode-factory.db?_pragma=busy_timeout(5000)"
	db           *sql.DB
)

func (p *program) Start(s service.Service) error {
	go p.Run()
	return nil
}

func (p *program) Run() {
	var err error
	db, err = sql.Open("sqlite", databasefile)
	if err != nil {
		logger.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	if err := initdb(); err != nil {
		logger.Fatalf("failed to prepare database: %v", err)
	}

	launchApi()
	go cropManager()
	mainLoop()
}

func (p *program) Stop(s service.Service) error {
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
				logger.Fatal("failed to cleanup job: %q", err)
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
	ct := make(chan struct{}, 2)
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

		ct <- struct{}{}
		go func(tj *TranscodeJob) {
			err := updateSourceMetadata(tj)
			if err != nil {
				logger.Errorf("job id %d: failed to determine source metadata: %q", tj.Id, err)
			}

			err = compileVF(tj)
			if err != nil {
				logger.Errorf("job id %d: failed to compile vf: %q", tj.Id, err)
			}
			updateJobStatus(tj.Id, AwaitingTranscode)
			err = deactivateJob(tj.Id)
			if err != nil {
				logger.Errorf("job id %d: failed to deactivate job: %q", tj.Id, err)
			}
			<-ct
		}(&tj)
	}
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
