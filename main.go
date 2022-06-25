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
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"database/sql"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/logger"
)

type TranscodeRequest struct {
	Source        string   `json:"source"`
	Destination   string   `json:"destination"`
	Srt_files     []string `json:"srt_files"`
	Crf           int      `json:"crf"`
	Autocrop      bool     `json:"autocrop"`
	Video_filters string   `json:"video_filters"`
	Audio_filters string   `json:"audio_filters"`
}

type JobState int

const (
	Submitted JobState = iota
	QueryingSource
	BuildVideoFilter
	BuildAudioFilter
	Transcoding
	Complete
	Failed
)

type TranscodeJob struct {
	Id            int
	JobDefinition TranscodeRequest
	PID           int
	State         JobState
}

var (
	databasefile = "//citadel.somuchcrypto.com/media/other/transcode-factory.db"
)

func initdb() error {
	db, err := sql.Open("sqlite", databasefile)
	if err != nil {
		return fmt.Errorf("failed to connect to db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
  CREATE TABLE IF NOT EXISTS transcode_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT,
    destination TEXT,
    crf INTEGER,
    srt_files BLOB,
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
    heartbeat BLOB,
    id INTEGER PRIMARY KEY,
    FOREIGN KEY (id)
      REFERENCES transcode_queue (id)
  );
  
  CREATE TABLE IF NOT EXISTS completed_jobs (
    id INTEGER PRIMARY KEY,
    source TEXT,
    destination TEXT,
    autocrop INTEGER,
    srt_files BLOB,
    ffmpegargs BLOB
    );
    `); err != nil {
		return err
	}
	return nil
}

func run() {
	db, err := sql.Open("sqlite", databasefile)
	if err != nil {
		logger.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	niq := `
  SELECT id, IFNULL(autocrop,1) as autocrop
  FROM transcode_queue
  WHERE id NOT IN (SELECT id FROM completed_jobs)
    AND id NOT IN (SELECT id FROM active_job)
  ORDER BY id ASC
  LIMIT 1;`

	for {
		r := db.QueryRow(niq)
		var tj TranscodeJob

		if err := r.Scan(&tj.Id, &tj.JobDefinition.Autocrop); err == sql.ErrNoRows {
			time.Sleep(10 * time.Second)
			continue
		}

		logger.Infof("job id %d: beginning processing", tj.Id)
		if err := updatejobstatus(db, tj.Id, Submitted); err != nil {
			logger.Errorf("failed to mark job active: %v", err)
			continue
		}

		logger.Infof("job id %d: determining length in frames", tj.Id)
		if err := updatetotalframes(db, tj.Id); err != nil {
			logger.Errorf("failed to determine number of frames in file: %v", err)
			continue
		}

		logger.Infof("job id %d: building video filter graph", tj.Id)
		updatejobstatus(db, tj.Id, BuildVideoFilter)
		if tj.JobDefinition.Autocrop {
			if err := addCrop(db, tj.Id); err != nil {
				logger.Errorf("updatefilters failed on job %d: %q", tj.Id, err)
			}
		} else {
			tx, err := db.Begin()
			if err != nil {
				logger.Errorf("failed to begin transaction: %q", err)
			}
			_, err = tx.Exec("UPDATE active_job SET vfilter = (SELECT video_filters FROM transcode_queue WHERE id =?)", tj.Id)
			if err != nil {
				logger.Errorf("failed to copy video filters to active job: %q, rollback: %q", err, tx.Rollback())
			}
			if err = tx.Commit(); err != nil {
				logger.Errorf("failed to commit transaction: %q", err)
				tx.Rollback()
			}
		}

		logger.Infof("job id %d: beginning transcode", tj.Id)
		updatejobstatus(db, tj.Id, Transcoding)
		// transcodefile(db, tj)

	}
}

func launchapi() {
	http.HandleFunc("/statusz", display_rows)
	http.HandleFunc("/enqueue", newtranscode)
	go http.ListenAndServe(":51218", nil)
}

func main() {
	logger.Init("transcode-factory", true, true, ioutil.Discard)
	if err := initdb(); err != nil {
		logger.Fatalf("failed to prepare database: %v", err)
	}
	launchapi()
	run()
}
