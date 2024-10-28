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
	ActiveJobs     []TranscodeJob
	TranscodeQueue []TranscodeJob
	CompletedJobs  []TranscodeJob
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func display_rows(w http.ResponseWriter, req *http.Request) {
	// setup required variables
	var srtj []byte
	page := PageData{}
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Commit()

	// first query for queue items
	q, err := tx.Query(`
  SELECT id, source, destination, codec, IFNULL(crf,18), IFNULL(autocrop,1), srt_files 
  FROM transcode_queue
	WHERE id not in (SELECT id FROM active_jobs)
  ORDER BY id ASC
  `)
	if err != nil {
		fmt.Fprintf(w, "%+v", err)
	}
	defer q.Close()

	// parse queue into datastructure
	for q.Next() {
		var jobRow TranscodeJob
		err := q.Scan(&jobRow.Id, &jobRow.JobDefinition.Source, &jobRow.JobDefinition.Destination, &jobRow.JobDefinition.Codec, &jobRow.JobDefinition.Crf, &jobRow.JobDefinition.Autocrop, &srtj)
		if err != nil && err != sql.ErrNoRows {
			fmt.Fprintf(w, "fatal error scanning db response for queue: %#v", err)
			return
		}

		if err := json.Unmarshal(srtj, &jobRow.JobDefinition.Srt_files); err != nil {
			logger.Error("failed to unmarshall queue srt file")
		}

		page.TranscodeQueue = append(page.TranscodeQueue, jobRow)
	}
	q.Close()

	// query for active jobs
	a, err := tx.Query(`
	SELECT 
		transcode_queue.id,
		source,
		destination,
		IFNULL(job_state,0),
		IFNULL(video_filters, 'empty'),
		srt_files,
		IIF(transcode_queue.codec = 'copy', 0, crf),
		IFNULL(source_metadata.codec, 'unknown') as source_codec,
		IFNULL(transcode_queue.codec, 'libx265') as destination_codec
	FROM transcode_queue
		JOIN (active_jobs 
			LEFT JOIN source_metadata 
				ON source_metadata.id = active_jobs.id)
			ON transcode_queue.id = active_jobs.id`)
	if err != nil {
		logger.Errorf("error fetching active jobs: %q", err)
		return
	}
	defer a.Close()
	for a.Next() {
		var r TranscodeJob
		err = a.Scan(&r.Id, &r.JobDefinition.Source, &r.JobDefinition.Destination, &r.State, &r.JobDefinition.Video_filters, &srtj, &r.JobDefinition.Crf, &r.SourceMeta.Codec, &r.JobDefinition.Codec)

		if err := json.Unmarshal(srtj, &r.JobDefinition.Srt_files); err != nil {
			logger.Error("failed to unmarshall queue srt file")
		}

		page.ActiveJobs = append(page.ActiveJobs, r)
	}
	if err != nil && err != sql.ErrNoRows {
		fmt.Fprintf(w, "fatal error scanning db response for active job: %#v", err)
		return
	}

	// page.QueueLength = len(page.TranscodeQueue)
	t, err := template.New("results").Parse(html_template)
	if err != nil {
		logger.Errorf("fatal error parsing template: %#v", err)
	}

	if err := t.Execute(w, page); err != nil {
		p, _ := json.Marshal(page)
		logger.Errorf("template with data '%s' failed: %v,", p, err)
	}
}

func newtranscode(w http.ResponseWriter, req *http.Request) {
	db, err := sql.Open("sqlite", databasefile)
	if err != nil {
		fmt.Fprintf(w, "failed to connect to db: %v", err)
	}
	defer db.Close()
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, _ := i.LastInsertId()
	tx.Commit()
	logger.Infof("Added job id %d for %#v", id, j)
	fmt.Fprintf(w, `{"id": %d}`, id)
	wsHub.refresh <- true
}

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
