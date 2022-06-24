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
	sq := "SELECT source WHERE id = ?"
	rs := db.QueryRow(sq, id)
	var s string
	if err := rs.Scan(s); err != nil {
		return err
	}

	fc, err := countFrames(s)
	if err != nil {
		return fmt.Errorf("countFrames returned: %q", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	_, err = tx.Exec("UPDATE active_job SET total_frames = ? WHERE id = ?", fc, id)
	if err != nil {
		return fmt.Errorf("failed to update total_frames: %q, rollback: %q", err, tx.Rollback())
	}

	return tx.Commit()
}

func addCrop(db *sql.DB, id int) error {
	sq := "SELECT source WHERE id = ?"
	rs := db.QueryRow(sq, id)
	var s string
	if err := rs.Scan(s); err != nil {
		return err
	}

	fq := "SELECT video_filters FROM transcode_queue WHERE id = ?"
	rf := db.QueryRow(fq, id)
	var df string
	if err := rf.Scan(df); err != nil {
		return err
	}

	c, err := detectCrop(s)
	if err != nil {
		return err
	}

	vf := strings.Join([]string{c, df}, ";")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}
	_, err = tx.Exec("UPDATE active_job SET vfilter = ? WHERE id = ?", vf, id)
	if err != nil {
		return fmt.Errorf("failed to update vfilter: %q, rollback: %q", err, tx.Rollback())
	}
	return tx.Commit()
}
