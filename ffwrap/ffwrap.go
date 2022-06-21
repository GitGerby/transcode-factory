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
package ffwrap

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)
var (
	ffmpegbinary = "f:/ffmpeg/ffmpeg.exe"
	ffprobebinary = "f:/ffmpeg/ffprobe.exe"
	ffquiet = []string{"-hide_banner", "-loglevel", "error"}
	ffcommon = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
	cdregex = regexp.MustCompile('t:[\d]*.*?(crop=[-\d:]*)')
)
const (
)

func DetectCrop(s string) (string, error) {
	var args []string
	args = append(args, "-hide_banner", ffcommon...)
	args = append(args, "-i", s, "-vf", "cropdetect=round=2", "-t", "300", "-f", "null", "NUL")
	
	var sout, serr bytes.Buffer
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	cmd := exec.Command(ffmpegbinary,args...)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to exec cropdetect: %v",err)
	}
	if cmd.ProcessState.ExitCode() {
		return "", fmt.Errorf("ffmpeg exited with error: %v", serr.String())
	}
	m := cdregex.FindAllStringSubmatch(serr.String(), -1)
	return m[len(m)-1][1],nil
}

