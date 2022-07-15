// Copyright 2022 GearnsC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"github.com/google/logger"
)

type FfprobeOutput struct {
	Streams []FfprobeStreams
}

type FfprobeStreams struct {
	Codec  string `json:"codec_name"`
	Frames string `json:"nb_read_frames"`
}

const (
	// ffmpegbinary  = "X:/other/tools/ffmpeg-gyan/ffmpeg-2022-02-28-git-7a4840a8ca-full_build/bin/ffmpeg.exe"
	// ffprobebinary = "X:/other/tools/ffmpeg-gyan/ffmpeg-2022-02-28-git-7a4840a8ca-full_build/bin/ffprobe.exe"
	ffmpegbinary  = "f:/ffmpeg/ffmpeg.exe"
	ffprobebinary = "f:/ffmpeg/ffprobe.exe"
)

var (
	// cdregex extracts the correct crop filter from an ffmpeg cropdetect run
	cdregex = regexp.MustCompile(`t:([\d]*).*?(crop=[-\d:]*)`)
	// tcpregex extracts the current transcode progress from a running transcode
	tcpregex = regexp.MustCompile(`frame=[\s]*([\d]*).*speed=([\d\.x])*`)
	ffquiet  = []string{"-y", "-hide_banner", "-stats", "-loglevel", "error"}
	ffcommon = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
)

func parseFfmpegStats(p io.ReadCloser, j int) {
	scanner := bufio.NewScanner(p)
	for scanner.Scan() {
		m := tcpregex.FindAllStringSubmatch(scanner.Text(), -1)
		if m != nil {
			tx, err := db.Begin()
			if err != nil {
				continue
			}
			defer tx.Rollback()
			_, err = tx.Exec(`
			UPDATE active_job SET current_frame = ?, speed = ? WHERE id = ?
			`, m[len(m)-1][1], m[len(m)-1][2], j)
			if err != nil {
				logger.Errorf("failed to update job status: %#v", err)
				tx.Rollback()
			}
			tx.Commit()
		}
	}
}

func detectCrop(s string, hwaccel bool) (string, error) {
	var args []string
	args = append(args, "-hide_banner")
	args = append(args, ffcommon...)

	args = append(args, "-i", s, "-vf", "cropdetect=round=2", "-t", "300", "-f", "null", "NUL")

	var sout, serr bytes.Buffer
	cmd := exec.Command(ffmpegbinary, args...)
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
		"stream=codec_name", "-print_format", "json", s,
	}
	logger.Infof("calling ffprobe with: %#v", args)
	cmd := exec.Command(ffprobebinary, args...)
	o, err := cmd.CombinedOutput()
	if err != nil {
		return MediaMetadata{}, fmt.Errorf("failed to count frames: %v", err)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		return MediaMetadata{}, fmt.Errorf("ffprobe exited with nonzero exit code: %q", cmd.ProcessState.ExitCode())
	}

	var ffp FfprobeOutput
	json.Unmarshal(o, &ffp)
	/*
		nf, err := strconv.Atoi(ffp.Streams[0].Frames)
		if err != nil {
			logger.Errorf("%q", err)
		}
	*/
	return MediaMetadata{
		TotalFrames: 0,
		Codec:       ffp.Streams[0].Codec,
	}, nil
}

func ffmpegTranscode(tj TranscodeJob) ([]string, error) {
	args := append(ffquiet, ffcommon...)

	if strings.ToLower(tj.SourceMeta.Codec) == "vc1" {
		args = append(args, "-hwaccel", "auto")
	}

	args = append(args, "-i", tj.JobDefinition.Source)
	mapargs := []string{"-map", "0:m:language:eng?"}

	for m, i := range tj.JobDefinition.Srt_files {
		args = append(args, "-i", i)
		mapargs = append(mapargs, "-map", fmt.Sprintf("%d", m+1), "-metadata:s:s", "language=eng")
	}
	if strings.ToLower(tj.JobDefinition.Codec) != "copy" {
		args = append(args, "-vf", tj.JobDefinition.Video_filters)
	}
	args = append(args, buildCodec(tj.JobDefinition.Codec, tj.JobDefinition.Crf)...)
	args = append(args, "-c:a", "copy", "-c:s", "copy", "-c:t", "copy")
	args = append(args, mapargs...)
	args = append(args, tj.JobDefinition.Destination)

	cmd := exec.Command(ffmpegbinary, args...)

	/*
		ep, err := cmd.StderrPipe()
		if err != nil {
			logger.Errorf("failed to setup stderr pipe: %q", err)
		}
		go parseFfmpegStats(ep, tj.Id)
	*/

	logger.Infof("calling ffmpeg with args: %#v", args)
	err := cmd.Run()
	if err != nil {
		co, _ := cmd.CombinedOutput()
		return nil, fmt.Errorf("ffmpeg exec error: %q, %q", err, string(co))
	}
	return args, nil
}

func buildCodec(codec string, crf int) []string {
	switch strings.ToLower(codec) {
	case "copy":
		return []string{"-c:v", "copy"}
	default:
		return []string{
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
	}
}
