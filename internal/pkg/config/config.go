package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type TFConfig struct {
	TranscodeLimit *int
	CropLimit      *int
	CopyLimit      *int
	DBPath         *string
	FfmpegPath     *string
	FfprobePath    *string
	LogDirectory   *string
}

func ParseConfig(path string) *TFConfig {
	c, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	config := &TFConfig{}
	err = yaml.Unmarshal(c, config)
	if err != nil {
		return nil
	}
	return config
}
