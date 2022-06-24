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
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

const (
	ffmpegbinary  = "f:/ffmpeg/ffmpeg.exe"
	ffprobebinary = "f:/ffmpeg/ffprobe.exe"
)

var (
	cdregex  = regexp.MustCompile(`t:([\d]*).*?(crop=[-\d:]*)`)
	ffquiet  = []string{"-hide_banner", "-loglevel", "error"}
	ffcommon = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
)

func detectCrop(s string) (string, error) {
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
	if len(m[len(m)-1]) < 2 {
		return "", fmt.Errorf("failed to extract crop string")
	}
	return string(m[len(m)-1][2]), nil
}

func countFrames(s string) (int, error) {
	args := []string{
		"-v", "error", "-select_stream", "v:0", "-count_frames", "-show_entries",
		"stream=nb_read_frames ", "-print_format", "default=nokey=1:noprint_wrappers=1",
	}
	cmd := exec.Command(ffprobebinary, args...)
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("failed to count frames: %v", err)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		return 0, fmt.Errorf("ffprobe exited with nonzero exit code: %q", cmd.ProcessState.ExitCode())
	}
	o, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to obtain output from ffprobe: %q", err)
	}
	return strconv.Atoi(string(o))
}
