package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	ListenPort     *int    `yaml:"listen_port,omitempty"`
}

const (
	defaultTranscodeLimit = 2
	defaultCropLimit      = 2
	defaultCopyLimit      = 4
	defaultListenPort     = 51218
)

var ErrYamlError = errors.New("error unmarshalling config file: ")

// Parse reads the config file and sets the config values
func (c *TFConfig) Parse(path string) error {
	// This is mostly a wrapper around loadConfig to allow for easier testing.
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return c.loadConfig(f)
}

// loadConfig reads the config file and sets the config values
func (c *TFConfig) loadConfig(configFile fs.File) error {
	f, err := io.ReadAll(configFile)
	if err != nil {
		return err
	}

	tempConfig := new(TFConfig)

	err = yaml.Unmarshal(f, tempConfig)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrYamlError, err)
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
	if tempConfig.ListenPort == nil {
		c.ListenPort = new(int)
		*c.ListenPort = defaultListenPort
	} else {
		c.ListenPort = tempConfig.ListenPort
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

func (c *TFConfig) WriteConfig(path string) error {
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
