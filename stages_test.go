package main

import (
	"testing"
)

func TestDeactivateJob(t *testing.T) {
	odb := db
	db = createEmptyTestDb(t)
	t.Cleanup(func() {
		db.Close()
		db = odb
	})
	insertQueuedJob(t, 1, "libx265")

	err := deactivateJob(1)
	if err != nil {
		t.Errorf("failed to deactivate job: %v", err)
	}

	a, err := queryActive()
	if err != nil {
		t.Errorf("query active failed: %v", err)
	}
	if len(a) > 0 {
		t.Errorf("active job remains after deactivation: %#v", a)
	}
}

func TestUpdateJobStatus(t *testing.T) {
	odb := db
	db = createEmptyTestDb(t)
	oh := wsHub
	wsHub = newHub()
	t.Cleanup(func() {
		db.Close()
		db = odb
		wsHub = oh
	})
	insertQueuedJob(t, 1, "libx265")
	t.Run("update queued job", func(t *testing.T) {
		err := updateJobStatus(1, JOB_METADATA)
		if err != nil {
			t.Errorf("failed to update job status: %v", err)
		}

		a, err := queryActive()
		if err != nil {
			t.Errorf("query active failed: %v", err)
		}
		if len(a) != 1 {
			t.Errorf("job was not marked active")
		}
		if a[0].State != JOB_METADATA {
			t.Errorf("unexpected job state:%v", a[0].State)
		}
	})

	t.Run("update job that's already active", func(t *testing.T) {
		err := updateJobStatus(1, JOB_PENDINGTRANSCODE)
		if err != nil {
			t.Errorf("failed to update job status: %v", err)
		}
		a, err := queryActive()
		if err != nil {
			t.Errorf("query active failed: %v", err)
		}
		if len(a) != 1 {
			t.Errorf("job was not marked active")
		}
		if a[0].State != JOB_PENDINGTRANSCODE {
			t.Errorf("unexpected job state:%v", a[0].State)
		}

	})
}
