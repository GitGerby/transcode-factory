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

func (c *TFConfig) Parse(path string) error {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	err = yaml.Unmarshal(f, c)
	if err != nil {
		return err
	}

	if c.TranscodeLimit == nil {
		c.TranscodeLimit = new(int)
		*c.TranscodeLimit = 2
	}
	if c.CropLimit == nil {
		c.CropLimit = new(int)
		*c.CropLimit = 2
	}
	if c.CopyLimit == nil {
		c.CopyLimit = new(int)
		*c.CopyLimit = 4
	}
	if c.DBPath == nil {
		c.DBPath = new(string)
		*c.DBPath = defaultDBPath
	}
	if c.FfmpegPath == nil {
		c.FfmpegPath = new(string)
		*c.FfmpegPath = defaultFfmpegPath
	}
	if c.FfprobePath == nil {
		c.FfprobePath = new(string)
		*c.FfprobePath = defaultFfprobePath
	}
	if c.LogDirectory == nil {
		c.LogDirectory = new(string)
		*c.LogDirectory = defaultLogDirectory
	}
	return nil
}

func DefaultConfiguration() *TFConfig {
	dc := new(TFConfig)
	*dc.TranscodeLimit = 2
	*dc.CropLimit = 2
	*dc.CopyLimit = 4
	*dc.DBPath = defaultDBPath
	*dc.FfmpegPath = defaultFfmpegPath
	*dc.FfprobePath = defaultFfprobePath
	*dc.LogDirectory = defaultLogDirectory
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
