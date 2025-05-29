//go:build linux || darwin
// +build linux darwin

package config

const (
	// *nix & darwin defaults
	defaultFfmpegPath   = "/usr/bin/ffmpeg"
	defaultFfprobePath  = "/usr/bin/ffprobe"
	defaultLogDirectory = "/var/log/transcodefactory"
	defaultDBPath       = "/var/lib/transcodefactory/transcodefactory.db"

	DefaultConfigPath = "/etc/transcodefactory/config.yaml"
)
