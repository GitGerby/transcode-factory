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
	"os"
	"path/filepath"
	"strings"

	"github.com/google/logger"
)

func pullNextCrop() (TranscodeJob, error) {
	niq := `
  SELECT id, source, video_filters
  FROM transcode_queue 
	WHERE id NOT IN (SELECT id FROM completed_jobs)
		AND id NOT IN (SELECT id FROM active_job)
		AND autocrop = 1 
		AND crop_complete != 1
		AND codec != 'copy'
	ORDER BY id ASC
	LIMIT 1;`

	tj := TranscodeJob{
		JobDefinition: TranscodeRequest{Autocrop: true},
	}
	r := db.QueryRow(niq)
	err := r.Scan(&tj.Id, &tj.JobDefinition.Source, &tj.JobDefinition.Video_filters)
	if err == sql.ErrNoRows {
		return TranscodeJob{}, err
	} else if err != nil {
		return TranscodeJob{}, fmt.Errorf("db query error: %q", err)
	}
	return tj, nil
}

func pullNextTranscode() (TranscodeJob, error) {
	niq := `
  SELECT id, source, destination, IFNULL(crf,18) as crf, srt_files, IFNULL(autocrop,1) as autocrop, video_filters, audio_filters, codec 
  FROM transcode_queue
  WHERE id NOT IN (SELECT id FROM completed_jobs)
	AND ((autocrop = 1 AND crop_complete = 1) OR (autocrop = 0))
  ORDER BY id ASC
  LIMIT 1;`

	r := db.QueryRow(niq)
	var tj TranscodeJob
	var subs []byte
	err := r.Scan(&tj.Id, &tj.JobDefinition.Source, &tj.JobDefinition.Destination, &tj.JobDefinition.Crf, &subs, &tj.JobDefinition.Autocrop, &tj.JobDefinition.Video_filters, &tj.JobDefinition.Audio_filters, &tj.JobDefinition.Codec)
	if err == sql.ErrNoRows {
		return TranscodeJob{}, err
	} else if err != nil {
		return TranscodeJob{}, fmt.Errorf("db query error: %q", err)
	}

	err = json.Unmarshal(subs, &tj.JobDefinition.Srt_files)
	if err != nil {
		logger.Errorf("failed to unmarshal srt files: %q", err)
	}
	return tj, nil
}

func deactivateJob(id int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM active_job WHERE id = ?", id)
	return tx.Commit()
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

func querySourceTable(id int) (MediaMetadata, error) {
	tx, err := db.Begin()
	if err != nil {
		return MediaMetadata{}, fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()
	r := tx.QueryRow("SELECT codec, width, height FROM source_metadata WHERE id = ?", id)
	var m MediaMetadata
	err = r.Scan(m.Codec, m.Width, m.Height)
	if err == sql.ErrNoRows {
		return MediaMetadata{}, err
	} else if err != nil {
		return MediaMetadata{}, fmt.Errorf("failed to parse db response:%q", err)
	}
	return m, tx.Commit()
}

// updateSourceMetadata queries the database for existing ffprobe results; if
// none are found it runs ffprobe and populates the database and the provided
// struct.
func updateSourceMetadata(tj *TranscodeJob) error {
	m, err := querySourceTable(tj.Id)
	if err == nil {
		tj.SourceMeta = m
		return nil
	} else if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("querying source table failed: %q", err)
	}

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

	_, err = tx.Exec("UPDATE active_job SET total_frames = ?, source_codec = ? WHERE id = ?", fc.TotalFrames, fc.Codec, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update source metadata: %q", err)
	}
	_, err = tx.Exec("UPDATE source_metadata SET codec = ?, width = ?, height = ? WHERE id = ?", fc.Codec, fc.Width, fc.Height, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update source metadata: %q", err)
	}
	tj.SourceMeta.Codec = fc.Codec
	tj.SourceMeta.TotalFrames = fc.TotalFrames
	return tx.Commit()
}

func compileVF(tj *TranscodeJob) error {
	var h bool
	if strings.ToLower(tj.SourceMeta.Codec) == "vc1" {
		h = true
	} else {
		h = false
	}

	var c string
	if tj.JobDefinition.Autocrop {
		var err error
		c, err = detectCrop(tj.JobDefinition.Source, h)
		if err != nil {
			return err
		}
	}

	if tj.JobDefinition.Video_filters != "" && c != "" {
		tj.JobDefinition.Video_filters = strings.Join([]string{c, tj.JobDefinition.Video_filters}, ";")
	} else if c != "" {
		tj.JobDefinition.Video_filters = c
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()
	_, err = tx.Exec("UPDATE transcode_queue SET video_filters = ? WHERE id = ?", tj.JobDefinition.Video_filters, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to persist video_filters: %q", err)
	}
	_, err = tx.Exec("UPDATE active_job SET vfilter = ? WHERE id = ?", tj.JobDefinition.Video_filters, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update video_filters: %q", err)
	}
	_, err = tx.Exec("UPDATE transcode_queue SET crop_complete = 1 WHERE id = ?", tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update crop_complete: %q", err)
	}
	return tx.Commit()
}

func transcodeMedia(tj *TranscodeJob) ([]string, error) {
	// make sure the dest directory exists or create it

	logger.Infof("making path: %s", filepath.Dir(tj.JobDefinition.Destination))
	err := os.MkdirAll(filepath.Dir(tj.JobDefinition.Destination), 0664)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %q", err)
	}
	return ffmpegTranscode(*tj)
}

func finishJob(tj *TranscodeJob, args []string) error {
	cq := `
	INSERT INTO completed_jobs (id, source, destination, autocrop, ffmpegargs, status)
	VALUES(?, ?, ?, ?, ?, ?)
	`
	rm := `
	DELETE FROM transcode_queue WHERE id = ?;
	DELETE FROM active_job WHERE id = ?;
	`
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()

	complete, err := tx.Prepare(cq)
	if err != nil {
		return fmt.Errorf("failed to prepare sql: %v", err)
	}
	clean, err := tx.Prepare(rm)
	if err != nil {
		return fmt.Errorf("failed to prepare sql: %v", err)
	}

	a, err := json.Marshal(args)
	if err != nil {
		return err
	}
	_, err = complete.Exec(tj.Id, tj.JobDefinition.Source, tj.JobDefinition.Destination, tj.JobDefinition.Autocrop, a, tj.State)
	if err != nil {
		return fmt.Errorf("failed to add completion record: %v", err)
	}
	_, err = clean.Exec(tj.Id, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to remove job records: %v", err)
	}
	return tx.Commit()
}
