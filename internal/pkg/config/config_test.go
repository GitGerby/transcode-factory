package config

import (
	_ "embed"
	"reflect"
	"testing"
)

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
	}
	*df.TranscodeLimit = defaultTranscodeLimit
	*df.CropLimit = defaultCropLimit
	*df.CopyLimit = defaultCopyLimit
	*df.DBPath = defaultDBPath
	*df.FfmpegPath = defaultFfmpegPath
	*df.FfprobePath = defaultFfprobePath
	*df.LogDirectory = defaultLogDirectory
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
