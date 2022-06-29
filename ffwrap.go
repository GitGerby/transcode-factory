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

// dbUpdateWrite fulfills the write closer interface so we can write progress
// updates to the database as ffmpeg runs.
type dbUpdateWriter struct {
	CombinedOutput bytes.Buffer
	JobId          int
}

const (
	ffmpegbinary  = "X:/other/tools/ffmpeg-gyan/ffmpeg-2022-02-28-git-7a4840a8ca-full_build/bin/ffmpeg.exe"
	ffprobebinary = "X:/other/tools/ffmpeg-gyan/ffmpeg-2022-02-28-git-7a4840a8ca-full_build/bin/ffprobe.exe"
)

var (
	// cdregex extracts the correct crop filter from an ffmpeg cropdetect run
	cdregex = regexp.MustCompile(`t:([\d]*).*?(crop=[-\d:]*)`)
	// tcpregex extracts the current transcode progress from a running transcode
	tcpregex = regexp.MustCompile(`frame=[\s]*([\d]*)`)
	ffquiet  = []string{"-hide_banner", "-stats", "-loglevel", "error"}
	ffcommon = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
)

func (w dbUpdateWriter) Write(p []byte) (int, error) {
	//	logger.Infof("%s", p)
	tx, err := db.Begin()
	if err != nil {
		return w.CombinedOutput.Write(p)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE active_job SET current_frame = ? WHERE id = ?`)
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
	i, err := strconv.Atoi(string(f[len(f)-1][1]))
	if err != nil {
		logger.Errorf("%q", err)
		i = 0
	}
	_, err = stmt.Exec(i, w.JobId)
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
		"stream=nb_read_frames,codec_name", "-print_format", "json", s,
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

	nf, err := strconv.Atoi(ffp.Streams[0].Frames)
	if err != nil {
		logger.Errorf("%q", err)
	}
	return MediaMetadata{
		TotalFrames: nf,
		Codec:       ffp.Streams[0].Codec,
	}, nil
}

func ffmpegTranscode(tj TranscodeJob) error {
	args := append(ffquiet, ffcommon...)

	if strings.ToLower(tj.SourceMeta.Codec) == "vc1" {
		args = append(args, "-hwaccel", "auto")
	}

	args = append(args, "-i", tj.JobDefinition.Source)
	mapargs := []string{"-map", "0:m:language:eng?", "-map", "0:v:0"}

	for m, i := range tj.JobDefinition.Srt_files {
		args = append(args, "-i", i)
		mapargs = append(mapargs, "-map", fmt.Sprintf("%d", m), "-metadata:s:s", "lanugage=eng")
	}
	if strings.ToLower(tj.JobDefinition.Codec) != "copy" {
		args = append(args, "-vf", tj.JobDefinition.Video_filters)
	}
	args = append(args, buildCodec(tj.JobDefinition.Codec, tj.JobDefinition.Crf)...)
	args = append(args, "-c:a", "copy", "-c:s", "copy", "-c:t", "copy")
	args = append(args, mapargs...)
	args = append(args, tj.JobDefinition.Destination)

	cmd := exec.Command(ffmpegbinary, args...)
	wr := dbUpdateWriter{
		JobId: tj.Id,
	}
	cmd.Stdout = wr
	cmd.Stderr = wr

	logger.Infof("calling ffmpeg with args: %#v", args)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg exec error: %q, %q", err, wr.CombinedOutput.String())
	}
	return nil
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
			//			"-spatial_aq", "1",
			//			"-temporal_aq", "1",
			"-preset", "1",
			//			"-b_ref_mode", "2",
		}
	}
}
