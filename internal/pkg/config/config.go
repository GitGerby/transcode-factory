package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
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
	ListenAddress  *string `yaml:"listen_address,omitempty"`
}

const (
	defaultTranscodeLimit = 2
	defaultCropLimit      = 2
	defaultCopyLimit      = 4
	defaultListenPort     = 51218
	defaultListenAddress  = ""
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

	switch {
	case tempConfig.TranscodeLimit != nil:
		c.TranscodeLimit = tempConfig.TranscodeLimit
	default:
		c.TranscodeLimit = new(int)
		*c.TranscodeLimit = defaultTranscodeLimit
	}

	switch {
	case tempConfig.CropLimit != nil:
		c.CropLimit = tempConfig.CropLimit
	default:
		c.CropLimit = new(int)
		*c.CropLimit = defaultCropLimit
	}

	switch {
	case tempConfig.CopyLimit != nil:
		c.CopyLimit = tempConfig.CopyLimit
	default:
		c.CopyLimit = new(int)
		*c.CopyLimit = defaultCopyLimit
	}

	switch {
	case tempConfig.DBPath != nil:
		c.DBPath = tempConfig.DBPath
	default:
		c.DBPath = new(string)
		*c.DBPath = defaultDBPath
	}

	switch {
	case tempConfig.FfmpegPath != nil:
		c.FfmpegPath = tempConfig.FfmpegPath
	case os.Getenv("TF_FFMPEG") != "":
		c.FfmpegPath = new(string)
		*c.FfmpegPath = os.Getenv("TF_FFMPEG")
	case func() bool {
		_, err := exec.LookPath("ffmpeg")
		return err == nil
	}():
		c.FfmpegPath = new(string)
		// We know err will be nil here so we can safely drop it
		*c.FfmpegPath, _ = exec.LookPath("ffmpeg")
	default:
		c.FfmpegPath = new(string)
		*c.FfmpegPath = defaultFfmpegPath
	}

	switch {
	case tempConfig.FfprobePath != nil:
		c.FfprobePath = tempConfig.FfprobePath
	case os.Getenv("TF_FFPROBE") != "":
		c.FfprobePath = new(string)
		*c.FfprobePath = os.Getenv("TF_FFPROBE")
	case func() bool {
		_, err := exec.LookPath("ffprobe")
		return err == nil
	}():
		c.FfprobePath = new(string)
		// We know err will be nil here so we can safely drop it
		*c.FfprobePath, _ = exec.LookPath("ffprobe")
	default:
		c.FfprobePath = new(string)
		*c.FfprobePath = defaultFfprobePath
	}

	switch {
	case tempConfig.LogDirectory != nil:
		c.LogDirectory = tempConfig.LogDirectory
	default:
		c.LogDirectory = new(string)
		*c.LogDirectory = defaultLogDirectory
	}

	switch {
	case tempConfig.ListenPort != nil:
		c.ListenPort = tempConfig.ListenPort
	default:
		c.ListenPort = new(int)
		*c.ListenPort = defaultListenPort
	}

	switch {
	case tempConfig.ListenAddress != nil:
		c.ListenAddress = tempConfig.ListenAddress
	default:
		c.ListenAddress = new(string)
		*c.ListenAddress = defaultListenAddress
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
