package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	inMemoryDatabase = ":memory:?_pragma=busy_timeout(5000)"
	goodJson         = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}`
	noCodecJson      = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"video_filters":""}`
	twoObjectsJson   = `[{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""},{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}]`
	badCrfJson       = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":"a","srt_files":["/path/to/srt/1.srt","/path/to/srt/2.srt"],"codec":"libx265","video_filters":""}`
	nsrtsJson        = `{"source":"/path/to/source.mkv","destination":"/path/to/destination.mkv","autocrop":true,"crf":18,"codec":"libx265","video_filters":""}`
)

func ephemeralDbTest(t *testing.T) *sql.DB {
	t.Helper()
	var err error
	db, err := sql.Open("sqlite", inMemoryDatabase)
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
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(goodJson)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "bad crf",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(badCrfJson)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "slice submitted",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(twoObjectsJson)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusBadRequest,
			rc:       testChannel,
		},
		{
			desc:     "no codec",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noCodecJson)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
		{
			desc:     "no srts",
			request:  httptest.NewRequest("POST", "/add", strings.NewReader(noCodecJson)),
			recorder: httptest.NewRecorder(),
			respCode: http.StatusOK,
			rc:       testChannel,
		},
	}

	for _, tc := range testCases {
		// init temp in memory database
		odb := db
		db = ephemeralDbTest(t)

		addHandler(tc.recorder, tc.request, tc.rc)

		// cleanup temp in memory database
		db.Close()
		db = odb

		result := tc.recorder.Result()
		defer result.Body.Close()
		if result.StatusCode != tc.respCode {
			t.Errorf("%q: wrong HTTP response got: %v, want %v", tc.desc, result.StatusCode, tc.respCode)
			<-tc.rc
		}
	}
}
