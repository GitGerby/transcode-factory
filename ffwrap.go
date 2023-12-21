// Copyright 2022 GearnsC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/logger"
)

type FfprobeOutput struct {
	Streams []FfprobeStreams
}

type FfprobeStreams struct {
	Codec      string `json:"codec_name"`
	Codec_type string `json:"codec_type"`
	Frames     string `json:"nb_read_frames"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

var (
	// cdregex extracts the correct crop filter from an ffmpeg cropdetect run
	cdregex  = regexp.MustCompile(`t:([\d]*).*?(crop=[-\d:]*)`)
	ffquiet  = []string{"-y", "-hide_banner", "-stats"}
	ffcommon = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
)

func detectCrop(s string, hwaccel bool) (string, error) {
	var args []string
	args = append(args, "-hide_banner")
	args = append(args, ffcommon...)

	args = append(args, "-i", s, "-vf", "cropdetect=round=2", "-t", "300", "-f", "null", "NUL")

	var sout, serr bytes.Buffer
	logger.Infof("cropdetect with args %#v", args)
	cmd := exec.CommandContext(ctx, ffmpegbinary, args...)
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to exec cropdetect: %v", err)
	}
	m := cdregex.FindAllSubmatch(serr.Bytes(), -1)
	if len(m) < 1 {
		return "", fmt.Errorf("failed to extract crop string")
	}
	if len(m[len(m)-1]) < 3 {
		return "", fmt.Errorf("failed to extract crop string")
	}
	return string(m[len(m)-1][2]), nil
}

func probeMetadata(s string) (MediaMetadata, error) {
	args := []string{
		"-threads", "32", "-v", "error", "-select_streams", "v:0", "-show_entries",
		"stream=codec_name,width,height,", "-print_format", "json", s,
	}
	logger.Infof("calling ffprobe with: %#v", args)
	cmd := exec.CommandContext(ctx, ffprobebinary, args...)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return MediaMetadata{}, fmt.Errorf("failed to count frames: %v", err)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		return MediaMetadata{}, fmt.Errorf("ffprobe exited with nonzero exit code: %q", cmd.ProcessState.ExitCode())
	}

	var ffp FfprobeOutput
	json.Unmarshal(o, &ffp)

	return MediaMetadata{
		TotalFrames: 0,
		Codec:       ffp.Streams[0].Codec,
		Width:       ffp.Streams[0].Width,
		Height:      ffp.Streams[0].Height,
	}, nil
}

func ffmpegTranscode(tj TranscodeJob) ([]string, error) {
	args := append(ffquiet, ffcommon...)

	if strings.ToLower(tj.SourceMeta.Codec) == "vc1" {
		args = append(args, "-hwaccel", "auto")
	}

	args = append(args, "-i", tj.JobDefinition.Source)

	mapargs := []string{"-map", "0:v:0", "-map", "0:a:m:language:eng?", "-map", "0:s:m:language:eng?", "-map", "0:t?"}
	if len(tj.JobDefinition.Srt_files) > 0 {
		for m, i := range tj.JobDefinition.Srt_files {
			if len(i) > 0 {
				args = append(args, "-i", i)
				mapargs = append(mapargs, "-map", fmt.Sprintf("%d", m+1), "-metadata:s:s", "language=eng")
			}
		}
	}
	if strings.ToLower(tj.JobDefinition.Codec) != "copy" && tj.JobDefinition.Video_filters != "" {
		args = append(args, "-vf", tj.JobDefinition.Video_filters)
	}
	args = append(args, buildCodec(tj.JobDefinition.Codec, tj.JobDefinition.Crf)...)
	args = append(args, "-c:a", "copy", "-c:s", "copy", "-c:t", "copy")
	args = append(args, mapargs...)
	args = append(args, tj.JobDefinition.Destination)

	_, fp := filepath.Split(tj.JobDefinition.Destination)
	logdest := filepath.Join(transcode_log_path, fmt.Sprintf("%s_%d.log", fp, time.Now().UnixNano()))
	log, err := os.Create(logdest)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	cmd := exec.CommandContext(ctx, ffmpegbinary, args...)
	cmd.Stderr = log
	cmd.Stdout = log
	logger.Infof("calling ffmpeg with args: %#v", args)
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %q", err)
	}
	err = cmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("execution failed: %q check log at %q", err, logdest)
	}

	return args, nil
}

func buildCodec(codec string, crf int) []string {
	libx265 := []string{
		"-c:v", "libx265",
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", "medium",
		"-profile:v", "main10",
		"-pix_fmt", "yuv420p10le",
	}
	libsvtav1 := []string{
		"-c:v", "libsvtav1",
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", "7",
		"-svtav1-params", "tune=0:enable-overlays=1",
		"-pix_fmt", "yuv420p10le",
	}

	switch strings.ToLower(codec) {
	case "copy":
		return []string{"-c:v", "copy"}
	case "libsvtav1":
		return libsvtav1
	case "hevc_nvenc":
		return []string{
			"-pix_fmt", "p010le",
			"-c:v", "hevc_nvenc",
			"-rc", "1",
			"-cq", fmt.Sprintf("%d", crf),
			"-profile:v", "1",
			"-tier", "1",
			"-spatial_aq", "1",
			"-temporal_aq", "1",
			"-preset", "1",
			"-b_ref_mode", "2",
		}
	case "libx265":
		return libx265
	default:
		return libx265
	}
}
