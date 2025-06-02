package config

import (
	"os"
	"path/filepath"

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

const (
	defaultTranscodeLimit = 2
	defaultCropLimit      = 2
	defaultCopyLimit      = 4
)

func (c *TFConfig) Parse(path string) error {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	tempConfig := new(TFConfig)

	err = yaml.Unmarshal(f, tempConfig)
	if err != nil {
		return err
	}

	if tempConfig.TranscodeLimit == nil {
		c.TranscodeLimit = new(int)
		*c.TranscodeLimit = defaultTranscodeLimit
	} else {
		c.TranscodeLimit = tempConfig.TranscodeLimit
	}
	if tempConfig.CropLimit == nil {
		c.CropLimit = new(int)
		*c.CropLimit = defaultCropLimit
	} else {
		c.CropLimit = tempConfig.CropLimit
	}
	if tempConfig.CopyLimit == nil {
		c.CopyLimit = new(int)
		*c.CopyLimit = defaultCopyLimit
	} else {
		c.CopyLimit = tempConfig.CopyLimit
	}
	if tempConfig.DBPath == nil {
		c.DBPath = new(string)
		*c.DBPath = defaultDBPath
	} else {
		c.DBPath = tempConfig.DBPath
	}
	if tempConfig.FfmpegPath == nil {
		c.FfmpegPath = new(string)
		*c.FfmpegPath = defaultFfmpegPath
	} else {
		c.FfmpegPath = tempConfig.FfmpegPath
	}
	if tempConfig.FfprobePath == nil {
		c.FfprobePath = new(string)
		*c.FfprobePath = defaultFfprobePath
	} else {
		c.FfprobePath = tempConfig.FfprobePath
	}
	if tempConfig.LogDirectory == nil {
		c.LogDirectory = new(string)
		*c.LogDirectory = defaultLogDirectory
	} else {
		c.LogDirectory = tempConfig.LogDirectory
	}

	return nil
}

func DefaultConfiguration() *TFConfig {
	dc := new(TFConfig)
	err := yaml.Unmarshal(defaultConfig, dc)
	if err != nil {
		panic(err)
	}
	return dc
}

func (c *TFConfig) DumpConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0644); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Chmod(0644); err != nil {
		return err
	}

	b, err := yaml.Marshal(*c)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}
