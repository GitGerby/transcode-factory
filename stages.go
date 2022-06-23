package main

import (
	"database/sql"
	"fmt"
)

func updatejobstatus(db *sql.DB, id int, js JobState) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	i, err := tx.Prepare(`
	INSERT OR IGNORE INTO active_jobs (id)
	VALUES (?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare sql statement: %v", err)
	}
	defer i.Close()

	u, err := tx.Prepare(`
	UPDATE active_jobs
	SET job_state = ?
	WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare sql statement: %v", err)
	}
	defer u.Close()

	_, err = i.Exec(id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to add job to active_jobs table: %v", err)
	}
	_, err = u.Exec(js, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update job state: %v", err)
	}
	return tx.Commit()
}

func updatefilters(db *sql.DB) error {
	return nil
}
