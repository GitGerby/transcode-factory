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
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	template "html/template"

	"github.com/google/logger"
	"github.com/gorilla/websocket"
)

type PageData struct {
	ActiveJobs    []TranscodeJob
	QueuedJobs    []PageQueueInfo
	CompletedJobs []TranscodeJob
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type PageQueueInfo struct {
	Id            int
	JobDefinition TranscodeRequest
	SourceMeta    MediaMetadata
	State         JobState
	CropState     string
}

// queryQueued fetches all jobs that are currently queued (not in active_jobs) from the database.
// The function returns a slice of PageQueueInfo objects representing the queued jobs if successful, or an error if something goes wrong.
func queryQueued() ([]PageQueueInfo, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Commit()

	var queuedJobs []PageQueueInfo
	var srtJsonBlob []byte

	q, err := tx.Query(`
  SELECT id, 
		source,
		destination,
		codec,
		IFNULL(crf,18),
		CASE
			WHEN autocrop = 1 AND crop_complete = 1 THEN 'complete'
			WHEN autocrop = 1 AND crop_complete = 0 THEN 'pending'
			WHEN autocrop IS NULL THEN 'pending'
			ELSE 'false'
		END AS autocrop,
		srt_files 
  FROM transcode_queue
	WHERE id not in (SELECT id FROM active_jobs)
  ORDER BY id ASC
  `)
	if err != nil {
		return nil, err
	}
	defer q.Close()

	for q.Next() {
		var jobRow PageQueueInfo
		err := q.Scan(&jobRow.Id, &jobRow.JobDefinition.Source, &jobRow.JobDefinition.Destination, &jobRow.JobDefinition.Codec, &jobRow.JobDefinition.Crf, &jobRow.CropState, &srtJsonBlob)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed scanning rows: %v", err)
		}

		if err := json.Unmarshal(srtJsonBlob, &jobRow.JobDefinition.Srt_files); err != nil {
			logger.Error("failed to unmarshall queue srt file")
		}

		queuedJobs = append(queuedJobs, jobRow)
	}
	return queuedJobs, nil
}

// queryActive fetches all active transcode jobs from the database.
// The function returns a slice of TranscodeJob objects representing the active jobs if successful, or an error if something goes wrong.
func queryActive() ([]TranscodeJob, error) {
	var srtJsonBlob []byte
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Commit()

	var activeJobs []TranscodeJob

	a, err := tx.Query(`
	SELECT 
		transcode_queue.id,
		source,
		destination,
		IFNULL(job_state,0),
		IFNULL(video_filters, 'none'),
		srt_files,
		IIF(transcode_queue.codec = 'copy', 0, crf),
		IFNULL(source_metadata.codec, 'unknown') as source_codec,
		IFNULL(transcode_queue.codec, 'libx265') as destination_codec,
		IFNULL(source_metadata.duration, 'unknown') as duration
	FROM transcode_queue
		JOIN (active_jobs 
			LEFT JOIN source_metadata 
				ON source_metadata.id = active_jobs.id)
			ON transcode_queue.id = active_jobs.id`)
	if err != nil {
		return nil, fmt.Errorf("error fetching active jobs: %q", err)
	}
	defer a.Close()

	for a.Next() {
		var jobRow TranscodeJob
		err = a.Scan(&jobRow.Id, &jobRow.JobDefinition.Source, &jobRow.JobDefinition.Destination, &jobRow.State, &jobRow.JobDefinition.Video_filters, &srtJsonBlob, &jobRow.JobDefinition.Crf, &jobRow.SourceMeta.Codec, &jobRow.JobDefinition.Codec, &jobRow.SourceMeta.Duration)

		if err := json.Unmarshal(srtJsonBlob, &jobRow.JobDefinition.Srt_files); err != nil {
			logger.Error("failed to unmarshall queue srt source(s)")
		}

		activeJobs = append(activeJobs, jobRow)
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("fatal error scanning db response for active job: %#v", err)
	}
	return activeJobs, nil
}

func statuszHandler(w http.ResponseWriter, req *http.Request) {
	page := PageData{}
	var err error

	page.QueuedJobs, err = queryQueued()
	if err != nil {
		logger.Errorf("failed to retrieve queued jobs: %v", err)
	}
	page.ActiveJobs, err = queryActive()
	if err != nil {
		logger.Errorf("failed to retrieve active jobs: %v", err)
	}

	t, err := template.New("results").Parse(html_template)
	if err != nil {
		logger.Errorf("fatal error parsing template: %#v", err)
	}

	if err := t.Execute(w, page); err != nil {
		p, _ := json.Marshal(page)
		logger.Errorf("template with data '%s' failed: %v,", p, err)
	}
}

func addHandler(w http.ResponseWriter, req *http.Request, refreshChannel chan<- bool) {
	tx, err := db.Begin()
	if err != nil {
		logger.Errorf("failed to begin transaction: %q", err)
		return
	}

	stmt, err := tx.Prepare(`
  INSERT INTO transcode_queue(source, destination, crf, srt_files, autocrop, video_filters, audio_filters, codec)
  VALUES(?, ?, ?, ?, ?, ?, ?, ?)
  `)
	if err != nil {
		tx.Rollback()
		fmt.Fprintf(w, "failed to prepare sql: %v", err)
	}

	var j TranscodeRequest
	err = json.NewDecoder(req.Body).Decode(&j)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to decode request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if j.Source == "" || j.Destination == "" {
		tx.Rollback()
		http.Error(w, "{error: source or destination cannot be empty}", http.StatusBadRequest)
		return
	}

	s, err := json.Marshal(j.Srt_files)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(j.Codec) == 0 {
		j.Codec = "libx265"
	}

	i, err := stmt.Exec(j.Source, j.Destination, j.Crf, s, j.Autocrop, j.Video_filters, j.Audio_filters, j.Codec)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := i.LastInsertId()
	tx.Commit()
	logger.Infof("Added job id %d for %#v", id, j)
	fmt.Fprintf(w, `{"id": %d}`, id)
	refreshChannel <- true
}

// logStream upgrades an HTTP connection to a WebSocket and integrates it into the websocket hub.
// It creates a new Client instance with the upgraded connection and registers it with the websocket hub.
// The readPump and writePump goroutines are started for handling incoming and outgoing messages respectively.
func logStream(w http.ResponseWriter, r *http.Request) {
	wsconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Errorf("failed to upgrade websocket: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	hubClient := &Client{
		hub:  wsHub,
		conn: wsconn,
		send: make(chan statusMessage, 10),
	}
	hubClient.hub.register <- hubClient
	go hubClient.writePump()
	go hubClient.readPump()
}
