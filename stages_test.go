package main

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/gitgerby/transcode-factory/internal/pkg/ffwrap"
	"github.com/google/go-cmp/cmp"
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

func TestPullNextTranscode(t *testing.T) {
	odb := db
	oh := wsHub
	wsHub = newHub()
	t.Cleanup(func() {
		db = odb
		wsHub = oh
	})

	testCases := []struct {
		desc           string
		setup          func()
		expectedResult TranscodeJob
		expectedError  error
	}{
		{
			desc:           "empty queue",
			setup:          func() {},
			expectedResult: TranscodeJob{},
			expectedError:  sql.ErrNoRows,
		},
		{
			desc: "transcode in queue",
			setup: func() {
				insertQueuedJob(t, 1, "libx265")
			},
			expectedResult: TranscodeJob{
				Id: 1,
				JobDefinition: ffwrap.TranscodeRequest{
					Source:        "/path/to/source1.mkv",
					Destination:   "/path/to/destination1.mkv",
					Srt_files:     []string{"srt_file1"},
					Crf:           18,
					Codec:         "libx265",
					Video_filters: "",
					Autocrop:      false,
				},
			},
			expectedError: nil,
		},
		{
			desc: "multiple items in queue",
			setup: func() {
				insertQueuedJob(t, 1, "copy")
				insertQueuedJob(t, 2, "libx265")
			},
			expectedResult: TranscodeJob{
				Id: 2,
				JobDefinition: ffwrap.TranscodeRequest{
					Source:        "/path/to/source2.mkv",
					Destination:   "/path/to/destination2.mkv",
					Srt_files:     []string{"srt_file2"},
					Crf:           18,
					Codec:         "libx265",
					Video_filters: "",
					Autocrop:      false,
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			db = createEmptyTestDb(t)
			defer db.Close()

			tc.setup()

			job, err := pullNextTranscode()
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("%q: unexpected error got %v want %v", tc.desc, err, tc.expectedError)
			}
			diff := cmp.Diff(tc.expectedResult, job)
			if diff != "" {
				t.Errorf("%q: unexpected job pulled: %q", tc.desc, diff)
			}
		})
	}
}

func TestPullNextCrop(t *testing.T) {
	odb := db
	oh := wsHub
	wsHub = newHub()
	t.Cleanup(func() {
		db = odb
		wsHub = oh
	})

	testCases := []struct {
		desc           string
		setup          func()
		expectedResult TranscodeJob
		expectedError  error
	}{
		{
			desc:           "empty queue",
			setup:          func() {},
			expectedResult: TranscodeJob{},
			expectedError:  sql.ErrNoRows,
		},
		{
			desc: "transcode in queue",
			setup: func() {
				insertQueuedCrop(t, 1, "libx265")
			},
			expectedResult: TranscodeJob{
				Id: 1,
				JobDefinition: ffwrap.TranscodeRequest{
					Source:        "/path/to/source1.mkv",
					Video_filters: "",
					Autocrop:      true,
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			db = createEmptyTestDb(t)
			defer db.Close()

			tc.setup()

			job, err := pullNextCrop()
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("%q: unexpected error got %v want %v", tc.desc, err, tc.expectedError)
			}
			diff := cmp.Diff(tc.expectedResult, job)
			if diff != "" {
				t.Errorf("%q: unexpected job pulled: %s", tc.desc, diff)
			}
		})
	}
}

func TestPullNextCopy(t *testing.T) {
	odb := db
	oh := wsHub
	wsHub = newHub()
	t.Cleanup(func() {
		db = odb
		wsHub = oh
	})

	testCases := []struct {
		desc           string
		setup          func()
		expectedResult TranscodeJob
		expectedError  error
	}{
		{
			desc:           "empty queue",
			setup:          func() {},
			expectedResult: TranscodeJob{},
			expectedError:  sql.ErrNoRows,
		},
		{
			desc: "transcode in queue",
			setup: func() {
				insertQueuedJob(t, 1, "copy")
			},
			expectedResult: TranscodeJob{
				Id: 1,
				JobDefinition: ffwrap.TranscodeRequest{
					Source:        "/path/to/source1.mkv",
					Destination:   "/path/to/destination1.mkv",
					Srt_files:     []string{"srt_file1"},
					Crf:           18,
					Codec:         "copy",
					Video_filters: "",
					Autocrop:      false,
				},
			},
			expectedError: nil,
		},
		{
			desc: "multiple items in queue",
			setup: func() {
				insertQueuedJob(t, 1, "libx265")
				insertQueuedJob(t, 2, "copy")
			},
			expectedResult: TranscodeJob{
				Id: 2,
				JobDefinition: ffwrap.TranscodeRequest{
					Source:        "/path/to/source2.mkv",
					Destination:   "/path/to/destination2.mkv",
					Srt_files:     []string{"srt_file2"},
					Crf:           18,
					Codec:         "copy",
					Video_filters: "",
					Autocrop:      false,
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			db = createEmptyTestDb(t)
			defer db.Close()

			tc.setup()

			job, err := pullNextCopy()
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("%q: unexpected error got %v want %v", tc.desc, err, tc.expectedError)
			}
			diff := cmp.Diff(tc.expectedResult, job)
			if diff != "" {
				t.Errorf("%q: unexpected job pulled: %q", tc.desc, diff)
			}
		})
	}
}

func TestQuerySourceTable(t *testing.T) {
	testCases := []struct {
		desc           string
		setup          func()
		jobId          int
		expectedResult ffwrap.MediaMetadata
		expectedError  error
	}{
		{
			desc:           "empty",
			setup:          func() {},
			jobId:          1,
			expectedResult: ffwrap.MediaMetadata{},
			expectedError:  sql.ErrNoRows,
		},
		{
			desc: "job not present",
			setup: func() {
				insertQueuedJob(t, 2, "libx265")
			},
			jobId:          1,
			expectedResult: ffwrap.MediaMetadata{},
			expectedError:  sql.ErrNoRows,
		},
		{
			desc: "job found",
			setup: func() {
				insertQueuedJob(t, 1, "libx265")
			},
			jobId: 1,
			expectedResult: ffwrap.MediaMetadata{
				Width:  7680,
				Height: 4320,
				Codec:  "h264",
			},
			expectedError: nil,
		},
	}
	for _, tc := range testCases {
		db = createEmptyTestDb(t)
		tc.setup()
		mm, err := querySourceTable(tc.jobId)
		if err != tc.expectedError {
			t.Errorf("%s: querySourceTable(%d) got err: %v, want %v", tc.desc, tc.jobId, err, tc.expectedError)
		}
		diff := cmp.Diff(mm, tc.expectedResult)
		if diff != "" {
			t.Errorf("%s: querySourceTable(%d) differed from expected value: %s", tc.desc, tc.jobId, diff)
		}
		db.Close()
	}
}
