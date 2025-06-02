//go:build linux || darwin
// +build linux darwin

package config

import (
	_ "embed"
)

const (
	// *nix & darwin defaults
	defaultFfmpegPath   = "/usr/bin/ffmpeg"
	defaultFfprobePath  = "/usr/bin/ffprobe"
	defaultLogDirectory = "/var/log/transcodefactory"
	defaultDBPath       = "/var/lib/transcodefactory/transcodefactory.db"

	DefaultConfigPath     = "/etc/transcodefactory/config.yaml"
	DefaultConfigTestFile = "default.yaml"
)

//go:embed default.yaml
var defaultConfig []byte
