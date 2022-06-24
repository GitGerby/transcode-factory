package main

import (
	"database/sql"
	"fmt"
)

func updatejobstatus(db *sql.DB, id int, js JobState) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %q", err)
	}

	i, err := tx.Prepare(`
	INSERT OR IGNORE INTO active_jobs (id)
	VALUES (?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare sql statement: %q", err)
	}
	defer i.Close()

	u, err := tx.Prepare(`
	UPDATE active_jobs
	SET job_state = ?
	WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare sql statement: %q", err)
	}
	defer u.Close()

	_, err = i.Exec(id)
	if err != nil {
		return fmt.Errorf("failed to add job to active_jobs table: %q, rollback result: %q", err, tx.Rollback())
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

func updatefilters(db *sql.DB) error {
	return nil
}
