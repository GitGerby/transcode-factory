package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type TFConfig struct {
	TranscodeLimit *int    `yaml:"transcode_limit,omitempty"`
	CropLimit      *int    `yaml:"crop_limit,omitempty"`
	CopyLimit      *int    `yaml:"copy_limit,omitempty"`
	DBPath         *string `yaml:"db_path,omitempty"`
	FfmpegPath     *string `yaml:"ffmpeg_path,omitempty"`
	FfprobePath    *string `yaml:"ffprobe_path,omitempty"`
	LogDirectory   *string `yaml:"log_directory,omitempty"`
}

func ParseConfig(path string) *TFConfig {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	config := &TFConfig{}

	err = yaml.Unmarshal(f, config)
	if err != nil {
		return nil
	}

	if config.TranscodeLimit == nil {
		config.TranscodeLimit = new(int)
		*config.TranscodeLimit = 2
	}
	if config.CropLimit == nil {
		config.CropLimit = new(int)
		*config.CropLimit = 2
	}
	if config.CopyLimit == nil {
		config.CopyLimit = new(int)
		*config.CopyLimit = 4
	}
	if config.DBPath == nil {
		config.DBPath = new(string)
		*config.DBPath = defaultDBPath
	}
	if config.FfmpegPath == nil {
		config.FfmpegPath = new(string)
		*config.FfmpegPath = defaultFfmpegPath
	}
	if config.FfprobePath == nil {
		config.FfprobePath = new(string)
		*config.FfprobePath = defaultFfprobePath
	}
	if config.LogDirectory == nil {
		config.LogDirectory = new(string)
		*config.LogDirectory = defaultLogDirectory
	}

	return config
}
