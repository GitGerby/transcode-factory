//go:build windows
// +build windows

package config

const (
	// windows defaults
	defaultWinFfmpegPath   = `c:\ffmpeg\ffmpeg.exe`
	defaultWinFfprobePath  = `c:\ffmpeg\ffprobe.exe`
	defaultWinLogDirectory = `c:\ProgramData\transcodefactory\logs`
	defaultWinDBPath       = `c:\ProgramData\transcodefactory\transcoder.db`
)
