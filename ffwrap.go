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
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/google/logger"
)

type FfprobeOutput struct {
	Streams []FfprobeStreams
}

type FfprobeStreams struct {
	Codec  string `json:"codec_name"`
	Frames int    `json:"nb_read_frames"`
}

// dbUpdateWrite fulfills the write closer interface so we can write progress
// updates to the database as ffmpeg runs.
type dbUpdateWriter struct {
	CombinedOutput bytes.Buffer
}

const (
	ffmpegbinary  = "f:/ffmpeg/ffmpeg.exe"
	ffprobebinary = "X:/other/tools/ffmpeg-gyan/ffmpeg-2022-02-28-git-7a4840a8ca-full_build/bin/ffprobe.exe"
)

var (
	// cdregex extracts the correct crop filter from an ffmpeg cropdetect run
	cdregex = regexp.MustCompile(`t:([\d]*).*?(crop=[-\d:]*)`)
	// tcpregex extracts the current transcode progress from a running transcode
	tcpregex = regexp.MustCompile(`frame=[\s]*([\d]*)`)
	ffquiet  = []string{"-hide_banner", "-loglevel", "error"}
	ffcommon = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
)

func (w dbUpdateWriter) Write(p []byte) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return w.CombinedOutput.Write(p)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE active_job SET current_frame = ?`)
	if err != nil {
		return w.CombinedOutput.Write(p)
	}

	f := tcpregex.FindAllSubmatch(p, -1)
	if len(f) < 1 {
		return w.CombinedOutput.Write(p)
	}
	if len(f[len(f)-1]) < 2 {
		return w.CombinedOutput.Write(p)
	}
	_, err = stmt.Exec(strconv.Atoi(string(f[len(f)-1][1])))
	if err != nil {
		return w.CombinedOutput.Write(p)
	}
	tx.Commit()
	return w.CombinedOutput.Write(p)
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
		"-threads", "16", "-v", "error", "-select_streams", "v:0", "-count_frames", "-show_entries",
		"stream=nb_read_frames ", "-print_format", "default=nokey=1:noprint_wrappers=1", s,
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

	return MediaMetadata{
		TotalFrames: ffp.Streams[0].Frames,
		Codec:       ffp.Streams[0].Codec,
	}, nil
}

func transcodeMedia(tj TranscodeJob) error {
	args := append(ffquiet, ffcommon...)
	args = append(args, []string{"-i", tj.JobDefinition.Source}...)
	mapargs := []string{"-map", "0:m:language:eng?", "-map", "0:v:0"}

	for m, i := range tj.JobDefinition.Srt_files {
		args = append(args, "-i", i)
		mapargs = append(mapargs, "-map", fmt.Sprintf("%d", m), "-metadata:s:s", "lanugage=eng")
	}

	args = append(args, "-vf", tj.JobDefinition.Video_filters)
	args = append(args, buildCodec("placeholder", tj.JobDefinition.Crf)...)
	args = append(args, "-c:a", "copy", "-c:s", "copy", "-c:t", "copy")
	args = append(args, mapargs...)
	args = append(args, tj.JobDefinition.Destination)

	cmd := exec.Command(ffmpegbinary, args...)
	var wr dbUpdateWriter
	cmd.Stdout = wr
	cmd.Stderr = wr

	return nil
}

func buildCodec(codec string, crf int) []string {
	switch codec {
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
