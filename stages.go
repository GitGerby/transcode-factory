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
	"strings"

	"github.com/google/logger"
)

func pullNextJob() (TranscodeJob, error) {
	niq := `
  SELECT id, source, destination, IFNULL(crf,18) as crf, srt_files, IFNULL(autocrop,1) as autocrop, video_filters, audio_filters, codec 
  FROM transcode_queue
  WHERE id NOT IN (SELECT id FROM completed_jobs)
    AND id NOT IN (SELECT id FROM active_job)
  ORDER BY id ASC
  LIMIT 1;`

	r := db.QueryRow(niq)
	var tj TranscodeJob
	var subs []byte
	if err := r.Scan(&tj.Id, &tj.JobDefinition.Source, &tj.JobDefinition.Destination, &tj.JobDefinition.Crf, &subs, &tj.JobDefinition.Autocrop, &tj.JobDefinition.Video_filters, &tj.JobDefinition.Audio_filters, &tj.JobDefinition.Codec); err == sql.ErrNoRows {
		return TranscodeJob{}, err
	}

	err := json.Unmarshal(subs, &tj.JobDefinition.Srt_files)
	if err != nil {
		logger.Errorf("failed to unmarshal srt files: %q", err)
	}
	return tj, nil
}

func updateJobStatus(id int, js JobState) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()

	i, err := tx.Prepare(`
  INSERT OR IGNORE INTO active_job (id)
  VALUES (?)
  `)
	if err != nil {
		return fmt.Errorf("failed to prepare sql statement: %q", err)
	}
	defer i.Close()

	u, err := tx.Prepare(`
  UPDATE active_job
  SET job_state = ?
  WHERE id = ?
  `)
	if err != nil {
		return fmt.Errorf("failed to prepare sql statement: %q", err)
	}
	defer u.Close()

	_, err = i.Exec(id)
	if err != nil {
		return fmt.Errorf("failed to add job to active_job table: %q", err)
	}
	_, err = u.Exec(js, id)
	if err != nil {
		return fmt.Errorf("failed to update job state: %q", err)
	}
	return tx.Commit()
}

func updateSourceMetadata(tj *TranscodeJob) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()

	sq := "SELECT source FROM transcode_queue WHERE id = ?;"
	rs := db.QueryRow(sq, tj.Id)
	var s string
	if err := rs.Scan(&s); err != nil {
		return fmt.Errorf("failed to query source file for index %q: %q", tj.Id, err)
	}

	fc, err := probeMetadata(s)
	if err != nil {
		return fmt.Errorf("countFrames returned: %q", err)
	}

	_, err = tx.Exec("UPDATE active_job SET total_frames = ?, codec = ? WHERE id = ?", fc.TotalFrames, fc.Codec, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update total_frames: %q", err)
	}
	tj.SourceMeta.Codec = fc.Codec
	tj.SourceMeta.TotalFrames = fc.TotalFrames
	return tx.Commit()
}

func addCrop(tj *TranscodeJob) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()

	sq := "SELECT source FROM transcode_queue WHERE id = ?"
	rs := db.QueryRow(sq, tj.Id)
	var s string
	if err := rs.Scan(&s); err != nil {
		return err
	}

	c, err := detectCrop(s)
	if err != nil {
		return err
	}

	tj.JobDefinition.Video_filters = strings.Join([]string{c, tj.JobDefinition.Video_filters}, ";")

	_, err = tx.Exec("UPDATE active_job SET vfilter = ? WHERE id = ?", tj.JobDefinition.Video_filters, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update vfilter: %q", err)
	}
	return tx.Commit()
}
