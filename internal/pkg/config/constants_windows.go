//go:build windows
// +build windows

package config

const (
	// windows defaults
	defaultFfmpegPath   = `c:\ffmpeg\ffmpeg.exe`
	defaultFfprobePath  = `c:\ffmpeg\ffprobe.exe`
	defaultLogDirectory = `c:\ProgramData\transcodefactory\logs`
	defaultDBPath       = `c:\ProgramData\transcodefactory\transcodefactory.db`

	DefaultConfigPath = `C:\ProgramData\transcodefactory\config.yaml`
)
