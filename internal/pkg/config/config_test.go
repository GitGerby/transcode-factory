package config

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//go:embed test_data/*
//go:embed default.yaml
//go:embed default_windows.yaml
var efs embed.FS

const (
	testFFProbeEnv = "/env/ffprobe"
	testFFMpegEnv  = "/env/ffmpeg"
)

func testFile(path string, t *testing.T) fs.File {
	t.Helper()
	f, err := efs.Open(path)
	if err != nil {
		t.Fatalf("failed to open file %s: %v", path, err)
	}
	return f
}

func buildFromConstants(t *testing.T) *TFConfig {
	t.Helper()

	df := &TFConfig{
		TranscodeLimit: new(int),
		CropLimit:      new(int),
		CopyLimit:      new(int),
		DBPath:         new(string),
		FfmpegPath:     new(string),
		FfprobePath:    new(string),
		LogDirectory:   new(string),
		ListenPort:     new(int),
		ListenAddress:  new(string),
	}

	*df.TranscodeLimit = defaultTranscodeLimit
	*df.CropLimit = defaultCropLimit
	*df.CopyLimit = defaultCopyLimit
	*df.DBPath = defaultDBPath
	*df.FfmpegPath = defaultFfmpegPath
	*df.FfprobePath = defaultFfprobePath
	*df.LogDirectory = defaultLogDirectory
	*df.ListenPort = defaultListenPort
	*df.ListenAddress = defaultListenAddress
	return df
}

func buildFromEnv(t *testing.T) *TFConfig {
	t.Helper()
	df := buildFromConstants(t)
	*df.FfmpegPath = testFFMpegEnv
	*df.FfprobePath = testFFProbeEnv
	return df
}

func TestDefaultConfiguration(t *testing.T) {
	tests := []struct {
		name string
		want *TFConfig
	}{
		{
			name: "default configuration",
			want: buildFromConstants(t),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultConfiguration(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultConfiguration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		testFile fs.File
		setup    func() error
		want     *TFConfig
		err      error
	}{
		{
			name:     "default configuration",
			testFile: testFile(DefaultConfigTestFile, t),
			want:     buildFromConstants(t),
		},
		{
			name:     "empty config file",
			testFile: testFile("test_data/empty.yaml", t),
			want:     buildFromConstants(t),
		},
		{
			name:     "empty config file with env",
			testFile: testFile("test_data/empty.yaml", t),
			setup: func() error {
				if err := os.Setenv("TF_FFMPEG", testFFMpegEnv); err != nil {
					return err
				}
				return os.Setenv("TF_FFPROBE", testFFProbeEnv)
			},
			want: buildFromEnv(t),
		},
		{
			name:     "invalid config file",
			testFile: testFile("test_data/invalid.yaml", t),
			want:     &TFConfig{},
			err:      ErrYamlError,
		},
	}

	tp := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", tp) }()
	ffme := os.Getenv("TF_FFMPEG")
	defer func() { os.Setenv("TF_FFMPEG", ffme) }()
	ffpe := os.Getenv("TF_FFPROBE")
	defer func() { os.Setenv("TF_FFPROBE", ffpe) }()
	os.Setenv("PATH", "")

	for _, tt := range tests {
		conf := new(TFConfig)
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			err := conf.loadConfig(tt.testFile)
			diff := cmp.Diff(conf, tt.want)
			if diff != "" {
				t.Errorf("loadConfig() diff: %v", cmp.Diff(conf, tt.want))
			}
			if err == nil && tt.err != nil {
				t.Errorf("loadConfig() err = <nil>, want %v", tt.err)
			}
			if err != nil && tt.err == nil {
				t.Errorf("loadConfig() err = %v, want <nil>", err)
			}
			if err != nil && tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Errorf("loadConfig() err = %v, want %v", err, tt.err)
				}
			}
		})
	}
}
