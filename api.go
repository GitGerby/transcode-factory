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

	"github.com/google/logger"
	template "github.com/google/safehtml/template"
)

type PageData struct {
	ActiveJob      TranscodeJob
	TranscodeQueue []TranscodeJob
	CompletedJobs  []TranscodeJob
	QueueLength    int
}

const html_template = `
<!DOCTYPE html>
<html>
<head>
<meta http-equiv="refresh" content="30">
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
	<h1>transcode-factory status</h1><br>
	<h2>Active Job</h2>
	<table>
		<tr>
			<td>
				Job ID: {{.ActiveJob.Id}} <br>
				Source: {{.ActiveJob.JobDefinition.Source}}<br>
				Subtitles: <ol>
				{{range .ActiveJob.JobDefinition.Srt_files}}
					<li>{{.}}</li>
				{{end}}
				</ol>
				Destination: {{.ActiveJob.JobDefinition.Destination}}<br>
				</td>
			<td>
				Stage: {{.ActiveJob.State}}<br>
				CRF: {{.ActiveJob.JobDefinition.Crf}}<br>
				Video Filter: {{.ActiveJob.JobDefinition.Video_filters}}<br>
			</td>
		</tr>
	</table>
	<h2>Current Queue</h2><br>
	QueueLength: {{.QueueLength}}
  <table>
    <tr>
      <th>Job ID</th>
      <th>Source</th>
      <th>Destination</th>
      <th>CRF</th>
      <th>autocrop</th>
      <th>SRT Files</th>
    </tr>
    {{range .TranscodeQueue}}
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

// const html_template = `{{ printf "%#v" .ActiveJob}}`

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
  SELECT id, source, destination, IFNULL(crf,18), IFNULL(autocrop,1), srt_files 
  FROM transcode_queue
	WHERE id not in (SELECT id FROM active_job)
  ORDER BY id ASC
  `)
	if err != nil {
		fmt.Fprintf(w, "%+v", err)
	}
	defer q.Close()

	// parse queue into datastructure
	for q.Next() {
		var r TranscodeJob
		err := q.Scan(&r.Id, &r.JobDefinition.Source, &r.JobDefinition.Destination, &r.JobDefinition.Crf, &r.JobDefinition.Autocrop, &srtj)
		if err != nil {
			fmt.Fprintf(w, "fatal error scanning db response for queue: %#v", err)
			return
		}

		if err := json.Unmarshal(srtj, &r.JobDefinition.Srt_files); err != nil {
			http.Error(w, "failed to unmarshall queue srt file", http.StatusInternalServerError)
		}

		page.TranscodeQueue = append(page.TranscodeQueue, r)
	}
	q.Close()

	a := tx.QueryRow(`
	SELECT transcode_queue.id, source, destination, job_state, IFNULL(current_frame,0), IFNULL(total_frames,0), IFNULL(vfilter,'empty'), srt_files, crf
	FROM transcode_queue JOIN active_job ON transcode_queue.id = active_job.id`)

	err = a.Scan(&page.ActiveJob.Id, &page.ActiveJob.JobDefinition.Source, &page.ActiveJob.JobDefinition.Destination, &page.ActiveJob.State, &page.ActiveJob.CurrentFrame, &page.ActiveJob.SourceMeta.TotalFrames, &page.ActiveJob.JobDefinition.Video_filters, &srtj, &page.ActiveJob.JobDefinition.Crf)
	if err != nil {
		fmt.Fprintf(w, "fatal error scanning db response for active job: %#v", err)
		return
	}
	if err := json.Unmarshal(srtj, &page.ActiveJob.JobDefinition.Srt_files); err != nil {
		http.Error(w, "failed to unmarshall active job srt file", http.StatusInternalServerError)
	}

	page.QueueLength = len(page.TranscodeQueue)
	t, err := template.New("results").Parse(html_template)
	if err != nil {
		fmt.Fprintf(w, "fatal error parsing template: %v+", err)
	}

	if err := t.Execute(w, page); err != nil {
		logger.Errorf("template failed: %q", err)
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
}