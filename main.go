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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"database/sql"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/logger"
	template "github.com/google/safehtml/template"
)

type TranscodeRequest struct {
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	Srt_files   []string `json:"srt_files"`
	Crf         int      `json:"crf"`
	Autocrop    bool     `json:"autocrop"`
	Filter      string   `json:"filter"`
}

type JobState int

const (
	Submitted JobState = iota
	CropDetect
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

const html_template = `
<!DOCTYPE html>
<html>
<head>
<style>
table, td, th {
  border: 1px solid;
}

table {
  width: 100%;
  border-collapse: collapse;
}
</style>
</head>
<body>
  <table>
    <tr>
      <th>Job ID</th>
      <th>Source</th>
      <th>Destination</th>
      <th>CRF</th>
      <th>autocrop</th>
      <th>SRT Files</th>
    </tr>
    {{range .}}
      <tr>
        <td>{{.Id}}</td>
        <td>{{.JobDefinition.Source}}</td>
        <td>{{.JobDefinition.Destination}}</td>
        <td>{{.JobDefinition.Crf}}</td>
        <td>{{.JobDefinition.Autocrop}}</td>
        <td>
        {{range .JobDefinition.Srt_files}}
        {{.}}<br>
        {{end}}</td>
      </tr>
    {{end}}
  </table>
</body>
`

func display_rows(w http.ResponseWriter, req *http.Request) {
	db, err := sql.Open("sqlite", databasefile)
	if err != nil {
		fmt.Fprintf(w, "failed to connect to db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, source, destination, IFNULL(crf,17), IFNULL(autocrop,1), srt_files FROM transcode_queue ORDER BY id ASC")
	if err != nil {
		fmt.Fprintf(w, "%v+", err)
	}
	defer rows.Close()

	var data []TranscodeJob
	for rows.Next() {
		var r TranscodeJob
		var srtj []byte
		err := rows.Scan(&r.Id, &r.JobDefinition.Source, &r.JobDefinition.Destination, &r.JobDefinition.Crf, &r.JobDefinition.Autocrop, &srtj)
		if err != nil {
			fmt.Fprintf(w, "fatal error scanning db response: %#v", err)
			return
		}

		json.Unmarshal(srtj, &r.JobDefinition.Srt_files)

		data = append(data, r)
	}
	rows.Close()

	t, err := template.New("results").Parse(html_template)
	if err != nil {
		fmt.Fprintf(w, "fatal error parsing template: %v+", err)
	}
	t.Execute(w, data)
}

func headers(w http.ResponseWriter, req *http.Request) {
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}

func newtranscode(w http.ResponseWriter, req *http.Request) {
	db, err := sql.Open("sqlite", databasefile)
	if err != nil {
		fmt.Fprintf(w, "failed to connect to db: %v", err)
	}
	defer db.Close()

	stmt, err := db.Prepare(`INSERT INTO transcode_queue(source, destination, srt_files, autocrop, filter) VALUES(?, ?, ?, ?, ?)`)
	if err != nil {
		fmt.Fprintf(w, "failed to prepare sql: %v", err)
	}
	defer stmt.Close()

	var j TranscodeRequest
	err = json.NewDecoder(req.Body).Decode(&j)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	s, err := json.Marshal(j.Srt_files)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	i, err := stmt.Exec(j.Source, j.Destination, s, j.Autocrop, j.Filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	fmt.Fprintf(w, "%#v", i)
}

func initdb() error {
	db, err := sql.Open("sqlite", databasefile)
	if err != nil {
		fmt.Printf("failed to connect to db: %v", err)
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
    id INTEGER,
    FOREIGN KEY (id)
      REFERENCES transcode_queue (id)
  );
  
  CREATE TABLE IF NOT EXISTS completed_jobs (
    id INTEGER PRIMARY KEY
    source TEXT,
    destination TEXT,
    autocrop INTEGER,
    srt_files BLOB,
    ffmpegargs BLOB;
    `); err != nil {
		return err
	}
	return nil
}

func run() {
	db, err := sql.Open("sqlite", databasefile)
	if err != nil {
		fmt.Printf("failed to connect to db: %v", err)
	}
	defer db.Close()

	niq := `
  SELECT id, source, destination, IFNULL(crf,17) as crf, IFNULL(autocrop,1) as autocrop, srt_files
  FROM transcode_queue
  WHERE id NOT IN (SELECT id FROM completed_jobs)
    AND id NOT IN (SELECT id FROM active_job)
  ORDER BY id ASC
  LIMIT 1;`

	for {
		r := db.QueryRow(niq)
		var tj TranscodeJob

		if err := r.Scan(&tj.Id); err == sql.ErrNoRows {
			time.Sleep(10 * time.Second)
			continue
		}

		if err := updatejobstatus(db, tj.Id, Submitted); err != nil {
			logger.Errorf("failed to mark job active: %v", err)
			continue
		}
	}
}

func launchapi() {
	http.HandleFunc("/status", display_rows)
	http.HandleFunc("/enqueue", newtranscode)
	http.HandleFunc("/headers", headers)
	go http.ListenAndServe(":51218", nil)
}

func main() {
	logger.Init("transcode-factory", false, true, ioutil.Discard)
	if err := initdb(); err != nil {
		fmt.Printf("init error: %v", err)
		os.Exit(1)
	}
	launchapi()
	run()
}
