package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	inMemoryDatabase        = ":memory:?_pragma=busy_timeout(5000)"
	fullRequestJsonSingle   = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}`
	fullRequestJsonSlice    = `[{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""},{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}]`
	noCodecJsonSingle       = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"video_filters":""}`
	noCodecJsonSlice        = `[{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"video_filters":""}]`
	badCrfJsonSingle        = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":"a","srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}`
	badCrfJsonSlice         = `[{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":"a","srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}]`
	noSrtsJsonSingle        = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"codec":"libx265","video_filters":""}`
	noSrtsJsonSlice         = `[{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"codec":"libx265","video_filters":""}]`
	noSourceJsonSingle      = `{"destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}`
	noSourceJsonSlice       = `[{"destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}]`
	noDestinationJsonSingle = `{"source":"/path/to/source.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}`
	noDestinationJsonSlice  = `[{"source":"/path/to/source.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}]`
)

// createEmptyTestDb initializes an in-memory SQLite database for testing purposes.
// It creates a new temporary memory database and ensures the necessary tables are initialized.
func createEmptyTestDb(t *testing.T) *sql.DB {
	t.Helper()
	var err error
	db, err := sql.Open("sqlite", inMemoryDatabase) // Opens an SQLite database in memory with specified pragmas.
	if err != nil {
		t.Fatalf("failed to open temp memory database: %v", err)
	}
	if err := initDbTables(db); err != nil {
		t.Fatalf("failed to create temp memory tables: %v", err)
	}
	return db
}

func TestAddHandler(t *testing.T) {
	testChannel := make(chan bool, 128)

	defer func() {
		close(testChannel)
	}()

	testCases := []struct {
		desc     string
		request  *http.Request
		recorder *httptest.ResponseRecorder
		respCode int
		rc       chan bool
	}{
		{
			desc:     "good post",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(fullRequestJsonSingle)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "bad crf",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(badCrfJsonSingle)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "slice submitted",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(fullRequestJsonSlice)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "no codec",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noCodecJsonSingle)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "no srts",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noCodecJsonSingle)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "no source",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noSourceJsonSingle)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "no destination",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noDestinationJsonSingle)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// init temp in memory database
			odb := db
			db = createEmptyTestDb(t)

			addHandler(tc.recorder, tc.request, tc.rc)

			// cleanup temp in memory database and make sure the channel stays empty
			db.Close()
			db = odb
			select {
			case <-tc.rc:
			default:
			}

			result := tc.recorder.Result()
			defer result.Body.Close()
			if result.StatusCode != tc.respCode {
				t.Errorf("%q: wrong HTTP response got: %v, want %v", tc.desc, result.StatusCode, tc.respCode)
			}
		})
	}
}

func TestBulkAddHandler(t *testing.T) {
	testChannel := make(chan bool, 128)

	defer func() {
		close(testChannel)
	}()

	testCases := []struct {
		desc     string
		request  *http.Request
		recorder *httptest.ResponseRecorder
		respCode int
		rc       chan bool
	}{
		{
			desc:     "good post",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(fullRequestJsonSlice)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "bad crf",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(badCrfJsonSlice)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "single submitted",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(fullRequestJsonSingle)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "no codec",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noCodecJsonSlice)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "no srts",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noCodecJsonSlice)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "no source",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noSourceJsonSlice)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "no destination",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noDestinationJsonSlice)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// init temp in memory database
			odb := db
			db = createEmptyTestDb(t)

			bulkAddHandler(tc.recorder, tc.request, tc.rc)

			// cleanup temp in memory database and make sure the channel stays empty
			db.Close()
			db = odb
			select {
			case <-tc.rc:
			default:
			}

			result := tc.recorder.Result()
			defer result.Body.Close()
			if result.StatusCode != tc.respCode {
				t.Errorf("%q: wrong HTTP response got: %v, want %v", tc.desc, result.StatusCode, tc.respCode)
			}
		})
	}
}

func TestQueryQueued(t *testing.T) {
	odb := db
	testChannel := make(chan bool, 128)
	defer func() {
		db.Close()
		db = odb
		close(testChannel)
	}()
	testCases := []struct {
		desc          string
		testValues    []TranscodeJob
		expectedQueue []PageQueueInfo
		expectedError error
	}{
		{desc: "empty queue", testValues: nil, expectedError: nil},
		{
			desc: "autocrop item in queue",
			testValues: []TranscodeJob{{
				Id: 1,
				JobDefinition: TranscodeRequest{
					Source:      "/path/to/source.mkv",
					Destination: "/path/to/destination.mkv",
					Crf:         18,
					Autocrop:    true,
					Codec:       "libx265",
				},
			}},
			expectedQueue: []PageQueueInfo{
				{
					Id: 1,
					JobDefinition: TranscodeRequest{
						Source:      "/path/to/source.mkv",
						Destination: "/path/to/destination.mkv",
						Crf:         18,
						Codec:       "libx265",
					},
					CropState: "pending",
				},
			},
		},
		{
			desc: "non autocrop item in queue",
			testValues: []TranscodeJob{{
				Id: 1,
				JobDefinition: TranscodeRequest{
					Source:      "/path/to/source.mkv",
					Destination: "/path/to/destination.mkv",
					Crf:         18,
					Autocrop:    false,
					Codec:       "libx265",
				},
			}},
			expectedQueue: []PageQueueInfo{
				{
					Id: 1,
					JobDefinition: TranscodeRequest{
						Source:      "/path/to/source.mkv",
						Destination: "/path/to/destination.mkv",
						Crf:         18,
						Codec:       "libx265",
					},
					CropState: "disabled",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			db = createEmptyTestDb(t)
			defer db.Close()
			if len(tc.testValues) > 0 {
				for _, v := range tc.testValues {
					jsonInput, err := json.Marshal(v.JobDefinition)
					if err != nil {
						t.Errorf("%q: failed to setup test case with value: %v", tc.desc, v)
					}
					req := httptest.NewRequest("POST", "/add", bytes.NewReader(jsonInput))
					addHandler(httptest.NewRecorder(), req, testChannel)
				}
			}
			qq, err := queryQueued()
			if err != tc.expectedError {
				t.Errorf("%q: unexpected err value got: %v, want: %v", tc.desc, err, tc.expectedError)
			}

			diff := cmp.Diff(tc.expectedQueue, qq)
			if diff != "" {
				t.Errorf("%q: job definition diff: %v", tc.desc, diff)
			}
		})
	}
}
