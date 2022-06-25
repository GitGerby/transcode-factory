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

	rows, err := db.Query(`
  SELECT id, source, destination, IFNULL(crf,17), IFNULL(autocrop,1), srt_files 
  FROM transcode_queue 
  ORDER BY id ASC
  `)
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
  INSERT INTO transcode_queue(source, destination, crf, srt_files, autocrop, video_filters, audio_filters)
  VALUES(?, ?, ?, ?, ?, ?, ?)
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
	i, err := stmt.Exec(j.Source, j.Destination, j.Crf, s, j.Autocrop, j.Video_filters, j.Audio_filters)
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
