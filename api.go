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
	ActiveJobs     []TranscodeJob
	TranscodeQueue []TranscodeJob
	CompletedJobs  []TranscodeJob
}

const html_template = `
<!DOCTYPE html>
<html>
<head>
<meta http-equiv="refresh" content="10">
<style>
	table, td, th {
		border: 1px solid;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}
}

</style>
</head>
<body>
	<h1>transcode-factory status</h1><br>
	<h2>Active Jobs</h2>
	Currently running jobs: {{len .ActiveJobs}}
	<table>
		{{range .ActiveJobs}}
		<tr>
			<th style="text-align:left">
				Job ID:
			</th>
			<td>
				{{.Id}}
			</td>
			<th style="text-align:left">
				Stage:
			</th>
			<td>
				{{.State}}
			</td>
		</tr>
		<tr>
			<th style="text-align:left">
				Source:
			</th>
			<td> 
				{{.JobDefinition.Source}} 
			</td>
			<th style="text-align:left">
				Codec:
			</th>
			<td>
			{{.SourceMeta.Codec}} 
			</td>
		</tr>
		<tr>
			<th style="text-align:left">
				Destination:
			</th>
			<td>
				{{.JobDefinition.Destination}}<br>
			</td>
			<th style="text-align:left">
				Codec / Crf:
			</th>
			<td>
				{{.JobDefinition.Codec}} / {{.JobDefinition.Crf}}
			</td>
		</tr>
		<tr>
			<th style="text-align:left">
				Subtitles:
			</th>
			<td colspan = "0">
				<ol>
					{{range .JobDefinition.Srt_files}}
						<li>{{.}}</li>
					{{end}}
				</ol>
			</td>
			<th style="text-align:left">
				Video Filter:
			</th>
			<td>
				{{.JobDefinition.Video_filters}}
			</td>
		</tr>
		{{end}}
	</table>
	<h2>Current Queue</h2><br>
	Queue Length: {{len .TranscodeQueue}}
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
  SELECT id, source, destination, codec, IFNULL(crf,18), IFNULL(autocrop,1), srt_files 
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
		err := q.Scan(&r.Id, &r.JobDefinition.Source, &r.JobDefinition.Destination, &r.JobDefinition.Codec, &r.JobDefinition.Crf, &r.JobDefinition.Autocrop, &srtj)
		if err != nil && err != sql.ErrNoRows {
			fmt.Fprintf(w, "fatal error scanning db response for queue: %#v", err)
			return
		}

		if err := json.Unmarshal(srtj, &r.JobDefinition.Srt_files); err != nil {
			logger.Error("failed to unmarshall queue srt file")
		}

		page.TranscodeQueue = append(page.TranscodeQueue, r)
	}
	q.Close()

	// query for active jobs
	a, err := tx.Query(`
	SELECT 
		transcode_queue.id,
		source,
		destination,
		job_state,
		IFNULL(current_frame,0),
		IFNULL(total_frames,0),
		IFNULL(video_filters, "empty"),
		srt_files,
		crf,
		IFNULL(source_metadata.codec, "unknown") as source_codec,
		IFNULL(transcode_queue.codec, "hevc_nvenc") as destination_codec
	FROM transcode_queue
		JOIN (active_job 
			LEFT JOIN source_metadata 
				ON source_metadata.id = active_job.id)
			ON transcode_queue.id = active_job.id`)
	if err != nil {
		logger.Errorf("error fetching active jobs: %q", err)
		return
	}
	defer a.Close()
	for a.Next() {
		var r TranscodeJob
		err = a.Scan(&r.Id, &r.JobDefinition.Source, &r.JobDefinition.Destination, &r.State, &r.CurrentFrame, &r.SourceMeta.TotalFrames, &r.JobDefinition.Video_filters, &srtj, &r.JobDefinition.Crf, &r.SourceMeta.Codec, &r.JobDefinition.Codec)

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
