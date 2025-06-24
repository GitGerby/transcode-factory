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
	"strconv"
	"strings"
	"time"

	"github.com/gitgerby/transcode-factory/internal/pkg/ffwrap"

	"github.com/google/logger"
)

// pullNextCrop retrieves the next crop job from the queue.
//
// It selects a job that is not yet completed or active, requires cropping
// (autocrop = 1), has not been cropped yet (crop_complete != 1), and does not
// use the 'copy' codec. The job details, including the source and video filters,
// are populated and returned as a TranscodeJob struct.
func pullNextCrop() (TranscodeJob, error) {
	niq := `
  SELECT id, source, video_filters
  FROM transcode_queue
	WHERE id NOT IN (SELECT id FROM completed_jobs)
		AND id NOT IN (SELECT id FROM active_jobs)
		AND autocrop = 1
		AND crop_complete != 1
		AND codec != 'copy'
	ORDER BY id ASC
	LIMIT 1;`

	tj := TranscodeJob{
		JobDefinition: ffwrap.TranscodeRequest{Autocrop: true},
	}
	r := db.QueryRow(niq)
	err := r.Scan(&tj.Id, &tj.JobDefinition.Source, &tj.JobDefinition.Video_filters)
	if err == sql.ErrNoRows {
		return TranscodeJob{}, err
	} else if err != nil {
		return TranscodeJob{}, fmt.Errorf("db query error: %w", err)
	}
	return tj, nil
}

// pullNextTranscode retrieves the next transcode job from the queue.
//
// It selects a job that is not yet completed, a copy, or active and is not
// waiting for autocrop. The job details returned as a TranscodeJob struct.
func pullNextTranscode() (TranscodeJob, error) {
	niq := `
  SELECT id, source, destination, IFNULL(crf,18) as crf, srt_files, IFNULL(autocrop,1) as autocrop, video_filters, audio_filters, codec
  FROM transcode_queue
  WHERE id NOT IN (SELECT id FROM completed_jobs)
	AND id NOT IN (SELECT id FROM active_jobs)
	AND ((autocrop = 1 AND crop_complete = 1) OR ((autocrop = 0) AND (LOWER(codec) != 'copy')))
  ORDER BY id ASC
  LIMIT 1;`

	r := db.QueryRow(niq)
	var tj TranscodeJob
	var subs []byte
	err := r.Scan(&tj.Id, &tj.JobDefinition.Source, &tj.JobDefinition.Destination, &tj.JobDefinition.Crf, &subs, &tj.JobDefinition.Autocrop, &tj.JobDefinition.Video_filters, &tj.JobDefinition.Audio_filters, &tj.JobDefinition.Codec)
	if err == sql.ErrNoRows {
		return TranscodeJob{}, err
	} else if err != nil {
		return TranscodeJob{}, fmt.Errorf("db query error: %w", err)
	}

	err = json.Unmarshal(subs, &tj.JobDefinition.Srt_files)
	if err != nil {
		logger.Errorf("failed to unmarshal srt files: %q", err)
	}
	return tj, nil
}

func pullNextCopy() (TranscodeJob, error) {
	niq := `
  SELECT id, source, destination, IFNULL(crf,18) as crf, srt_files, IFNULL(autocrop,1) as autocrop, video_filters, audio_filters, codec
  FROM transcode_queue
  WHERE id NOT IN (SELECT id FROM completed_jobs)
	AND id NOT IN (SELECT id FROM active_jobs)
	AND LOWER(codec) = 'copy'
  ORDER BY id ASC
  LIMIT 1;`

	r := db.QueryRow(niq)
	var tj TranscodeJob
	var subs []byte
	err := r.Scan(&tj.Id, &tj.JobDefinition.Source, &tj.JobDefinition.Destination, &tj.JobDefinition.Crf, &subs, &tj.JobDefinition.Autocrop, &tj.JobDefinition.Video_filters, &tj.JobDefinition.Audio_filters, &tj.JobDefinition.Codec)
	if err == sql.ErrNoRows {
		return TranscodeJob{}, err
	} else if err != nil {
		return TranscodeJob{}, fmt.Errorf("db query error: %w", err)
	}
	err = json.Unmarshal(subs, &tj.JobDefinition.Srt_files)
	if err != nil {
		logger.Errorf("failed to unmarshal srt files: %q", err)
	}
	return tj, nil
}

func deactivateJob(id int) error {
	_, err := db.Exec("DELETE FROM active_jobs WHERE id = ?", id)
	return err
}

// updateJobStatus sets the status of a job in the database and refreshes all connected status pages
func updateJobStatus(id int, js JobState) error {
	_, err := db.Exec(`
		INSERT INTO active_jobs (id, job_state)
			VALUES (?, ?)
			ON CONFLICT(id) DO UPDATE SET job_state=excluded.job_state
  `, id, js)
	if err != nil {
		return fmt.Errorf("failed to upsert active job %d with status %q: %v", id, js, err)
	}
	wsHub.refresh <- true
	return nil
}

// querySourceTable returns the media metadata or an error for a given job.
func querySourceTable(id int) (ffwrap.MediaMetadata, error) {
	tx, err := db.Begin()
	if err != nil {
		return ffwrap.MediaMetadata{}, fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()
	// IFNULL --> 8k resolution this ensures crops will trigger on basically any video if we don't detect the correct size
	r := tx.QueryRow("SELECT codec, IFNULL(width,7680), IFNULL(height,4320) FROM source_metadata WHERE id = ?", id)
	var m ffwrap.MediaMetadata
	err = r.Scan(&m.Codec, &m.Width, &m.Height)
	if err == sql.ErrNoRows {
		return ffwrap.MediaMetadata{}, err
	} else if err != nil {
		return ffwrap.MediaMetadata{}, fmt.Errorf("failed to parse db response:%q", err)
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
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("querying source table failed: %q", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
	INSERT OR IGNORE INTO source_metadata (id)
	VALUES (?)
	`, tj.Id)
	if err != nil {
		return fmt.Errorf("job id %d: failed to insert source_metadata: %q", tj.Id, err)
	}

	sq := "SELECT source FROM transcode_queue WHERE id = ?"
	rs := db.QueryRow(sq, tj.Id)
	var s string
	if err := rs.Scan(&s); err != nil {
		return fmt.Errorf("failed to query source file for index %q: %q", tj.Id, err)
	}

	fc, err := ffwrap.ProbeMetadata(ctx, s)
	if err != nil {
		return fmt.Errorf("metadata probe returned: %q", err)
	}

	_, err = tx.Exec("UPDATE source_metadata SET codec = ?, width = ?, height = ?, duration = ? WHERE id = ?", fc.Codec, fc.Width, fc.Height, fc.Duration, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update source metadata: %q", err)
	}
	tj.SourceMeta.Width = fc.Width
	tj.SourceMeta.Height = fc.Height
	tj.SourceMeta.Codec = fc.Codec
	tj.SourceMeta.Duration = fc.Duration
	logger.Infof("job id %d:source metadata: %#v", tj.Id, tj.SourceMeta)
	return tx.Commit()
}

// compileVF builds the appropriate video filter string based on the provided filter string
// and the autocrop setting if set to true.
func compileVF(tj *TranscodeJob) error {
	var cropFilter string
	if tj.JobDefinition.Autocrop {
		var err error
		cropFilter, err = ffwrap.DetectCrop(ctx, tj.JobDefinition.Source)
		if err != nil {
			return err
		}
	}

	// parse the crop filter
	cropSlice := strings.Split(cropFilter, ":")
	if len(cropSlice) < 2 {
		return fmt.Errorf("splitting crop filter %q for parsing failed", cropFilter)
	}
	cropWidth, err := strconv.Atoi(cropSlice[0])
	if err != nil {
		cropWidth = 0
	}
	cropHeight, err := strconv.Atoi(cropSlice[1])
	if err != nil {
		cropHeight = 0
	}

	// only add the crop filter if it's actually going to reduce the number of pixels running through the pipeline.
	if cropWidth != tj.SourceMeta.Width || cropHeight != tj.SourceMeta.Height {
		if tj.JobDefinition.Video_filters != "" && cropFilter != "" {
			tj.JobDefinition.Video_filters = strings.Join([]string{cropFilter, tj.JobDefinition.Video_filters}, ",")
		} else if cropFilter != "" {
			tj.JobDefinition.Video_filters = cropFilter
		}
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

	_, err = tx.Exec("UPDATE transcode_queue SET crop_complete = 1 WHERE id = ?", tj.Id)
	if err != nil {
		return fmt.Errorf("failed to update crop_complete: %q", err)
	}
	return tx.Commit()
}

func createDestinationParent(path string) error {
	// make sure the dest directory exists or create it
	logger.Infof("making path: %q", filepath.Dir(path))
	err := os.MkdirAll(filepath.Dir(path), 0664)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %q", err)
	}
	return nil
}

func transcodeMedia(tj *TranscodeJob) ([]string, error) {
	if err := createDestinationParent(tj.JobDefinition.Destination); err != nil {
		return nil, err
	}
	err := registerLogFile(tj)
	if err != nil {
		return nil, err
	}
	// run the transcoder
	return ffwrap.FfmpegTranscode(ctx, tj.JobDefinition)
}

func finishJob(tj *TranscodeJob, args []string) error {
	cq := `
	INSERT INTO completed_jobs (id, source, destination, autocrop, ffmpegargs, status)
	VALUES(?, ?, ?, ?, ?, ?)
	`
	rm := `
	DELETE FROM transcode_queue WHERE id = ?;
	DELETE FROM active_jobs WHERE id = ?;
	DELETE FROM source_metadata WHERE id = ?;
	`
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	defer func() {
		tx.Rollback()
		wsHub.refresh <- true
	}()

	a, err := json.Marshal(args)
	if err != nil {
		return err
	}
	_, err = tx.Exec(cq, tj.Id, tj.JobDefinition.Source, tj.JobDefinition.Destination, tj.JobDefinition.Autocrop, a, tj.State)
	if err != nil {
		return fmt.Errorf("failed to add completion record: %v", err)
	}
	_, err = tx.Exec(rm, tj.Id, tj.Id)
	if err != nil {
		return fmt.Errorf("failed to remove job records: %v", err)
	}

	return tx.Commit()
}

// registerLogFile registers a log file path for a given job ID.
// It inserts or replaces the file path in the 'log_files' table.
func registerLogFile(tj *TranscodeJob) error {
	fp := filepath.Base(tj.JobDefinition.Destination)
	tj.JobDefinition.LogDestination = filepath.Join(encodeLogDir, fmt.Sprintf("%s_%d.log", fp, time.Now().UnixNano()))

	err := os.MkdirAll(filepath.Dir(tj.JobDefinition.LogDestination), 0644)
	if err != nil {
		return fmt.Errorf("could not create directory for encoder log: %w", err)
	}

	_, err = db.Exec(`
		INSERT OR REPLACE INTO log_files(id, logfile)
		VALUES(?,?)
	`, tj.Id, tj.JobDefinition.LogDestination)
	if err != nil {
		return err
	}

	return nil
}
