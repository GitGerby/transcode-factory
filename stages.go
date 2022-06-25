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
	"fmt"
	"strings"
)

func updatejobstatus(db *sql.DB, id int, js JobState) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}

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
		return fmt.Errorf("failed to add job to active_job table: %q, rollback result: %q", err, tx.Rollback())
	}
	_, err = u.Exec(js, id)
	if err != nil {
		return fmt.Errorf("failed to update job state: %q, rollback result: %q", err, tx.Rollback())
	}
	return tx.Commit()
}

func updatetotalframes(db *sql.DB, id int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	sq := "SELECT source FROM transcode_queue WHERE id = ?;"
	rs := db.QueryRow(sq, id)
	var s string
	if err := rs.Scan(&s); err != nil {
		return fmt.Errorf("failed to query source file for index %q: %q", id, err)
	}

	fc, err := countFrames(s)
	if err != nil {
		return fmt.Errorf("countFrames returned: %q", err)
	}

	_, err = tx.Exec("UPDATE active_job SET total_frames = ? WHERE id = ?", fc, id)
	if err != nil {
		return fmt.Errorf("failed to update total_frames: %q, rollback: %q", err, tx.Rollback())
	}

	return tx.Commit()
}

func addCrop(db *sql.DB, id int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	sq := "SELECT source FROM transcode_queue WHERE id = ?"
	rs := db.QueryRow(sq, id)
	var s string
	if err := rs.Scan(&s); err != nil {
		return err
	}

	fq := "SELECT video_filters FROM transcode_queue WHERE id = ?"
	rf := db.QueryRow(fq, id)
	var df string
	if err := rf.Scan(&df); err != nil {
		return err
	}

	c, err := detectCrop(s)
	if err != nil {
		return err
	}

	vf := strings.Join([]string{c, df}, ";")

	_, err = tx.Exec("UPDATE active_job SET vfilter = ? WHERE id = ?", vf, id)
	if err != nil {
		return fmt.Errorf("failed to update vfilter: %q, rollback: %q", err, tx.Rollback())
	}
	return tx.Commit()
}
