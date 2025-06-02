//go:build windows
// +build windows

package config

import (
	_ "embed"
)

const (
	// windows defaults
	defaultFfmpegPath   = `C:\ffmpeg\ffmpeg.exe`
	defaultFfprobePath  = `C:\ffmpeg\ffprobe.exe`
	defaultLogDirectory = `C:\ProgramData\transcodefactory\logs`
	defaultDBPath       = `C:\ProgramData\transcodefactory\transcodefactory.db`

	DefaultConfigPath = `C:\ProgramData\transcodefactory\config.yaml`
)

//go:embed default_windows.yaml
var defaultConfig []byte
