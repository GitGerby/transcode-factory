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
package ffwrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	libcodec "github.com/gitgerby/transcode-factory/internal/pkg/ffwrap/codec"

	"github.com/google/logger"
)

var (
	// cdregex extracts the correct crop filter from an ffmpeg cropdetect run
	cdregex       = regexp.MustCompile(`t:([\d]*).*?(crop=[-\d:]*)`)
	ffquiet       = []string{"-y", "-hide_banner", "-stats", "-loglevel", "error"}
	ffcommon      = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
	ffmpegbinary  string
	ffprobebinary string
)

func SetBinaryLocations(ffmpeg, ffprobe string) {
	ffmpegbinary = ffmpeg
	ffprobebinary = ffprobe
}

// detectCrop uses the FFmpeg tool to automatically detect and return the crop filter string for an input video file.
// It constructs a command with arguments suitable for the FFmpeg cropdetect filter, runs the command, and processes its output
// to extract and return the detected crop parameters as a string. If any error occurs during this process, it returns an error message.
func DetectCrop(ctx context.Context, inputFile string) (string, error) {
	args := append([]string{"-hide_banner"}, ffcommon...)

	args = append(args, "-i", inputFile, "-vf", "cropdetect=round=2", "-t", "300", "-f", "null", "NUL")

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

// probeMetadata uses the FFprobe tool to retrieve metadata about an input video file.
// It constructs a command with arguments suitable for retrieving stream information and runs it using FFprobe.
// The output is then parsed as JSON and converted into a MediaMetadata struct, which includes details such as codec name, width, height, duration.
// If any error occurs during the process, it returns an appropriate error message along with zero values in the MediaMetadata structure.
func ProbeMetadata(ctx context.Context, source string) (MediaMetadata, error) {
	args := []string{
		"-threads", "32", "-v", "error", "-select_streams", "v:0", "-show_format", "-show_entries",
		"stream=codec_name,width,height,", "-print_format", "json", "-sexagesimal", source,
	}
	logger.Infof("calling ffprobe with: %#v", args)
	cmd := exec.CommandContext(ctx, ffprobebinary, args...)
	sto, err := cmd.Output()
	if err != nil && cmd.ProcessState.ExitCode() != 0 {
		return MediaMetadata{}, fmt.Errorf("%q ffprobe unexpect output: %v or exit code: %q", source, err, cmd.ProcessState.ExitCode())
	}

	var ffp FfprobeOutput
	if err := json.Unmarshal(sto, &ffp); err != nil {
		return MediaMetadata{}, fmt.Errorf("unmarshall ffprobe data %#v: %w", sto, err)
	}

	if len(ffp.Streams) != 1 {
		return MediaMetadata{}, fmt.Errorf("got %d streams in ffprobe output; expected 1", len(ffp.Streams))
	}

	return MediaMetadata{
		Duration: ffp.Format.Duration,
		Codec:    ffp.Streams[0].Codec,
		Width:    ffp.Streams[0].Width,
		Height:   ffp.Streams[0].Height,
	}, nil
}

// ffmpegTranscode transcodes media files using FFmpeg based on the provided TranscodeJob configuration.
// It constructs and executes an FFmpeg command with various options to handle video, audio, subtitles, and other metadata from the source file.
// The function supports copying streams where specified ('copy' codec), applying video filters if defined, and handling additional subtitle files specified in srt_files.
// It captures stderr output for logging purposes and returns the FFmpeg command arguments upon successful completion or an error otherwise.
func FfmpegTranscode(ctx context.Context, tr TranscodeRequest) ([]string, error) {
	args := append(ffquiet, ffcommon...)

	args = append(args, "-i", tr.Source)

	mapargs := []string{
		"-map", "0:v:0",
		"-map", "0:a:m:language:eng:?",
		"-map", "0:s:m:language:eng:?",
		"-map", "0:t:?"}

	if len(tr.Srt_files) > 0 {
		for m, i := range tr.Srt_files {
			if len(i) > 0 {
				args = append(args, "-i", i)
				mapargs = append(mapargs, "-map", fmt.Sprintf("%d", m+1), "-metadata:s:s", "language=eng")
			}
		}
	}
	if strings.ToLower(tr.Codec) != "copy" && tr.Video_filters != "" {
		args = append(args, "-vf", tr.Video_filters)
	}

	colorMeta, err := parseColorInfo(ctx, tr.Source)
	if err != nil {
		logger.Errorf("failed to parse color metadata: %v", err)
	}
	logger.Infof("got color metadata: %#v", colorMeta)
	args = append(args, buildCodec(tr.Codec, tr.Crf, colorMeta)...)
	args = append(args, "-c:a", "copy", "-c:s", "copy", "-c:t", "copy")
	args = append(args, mapargs...)
	args = append(args, tr.Destination)

	log, err := os.Create(tr.LogDestination)
	if err != nil {
		logger.Errorf("failed to start log file: %v", err)
	}

	cmd := exec.CommandContext(ctx, ffmpegbinary, args...)
	cmd.Dir = filepath.Dir(ffmpegbinary)
	cmd.Stderr = log
	cmd.Stdout = log
	logger.Infof("calling ffmpeg with args: %#v", args)
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %q", err)
	}

	err = cmd.Wait()
	if errors.Is(err, context.Canceled) {
		return nil, err
	} else if err != nil || cmd.ProcessState.ExitCode() != 0 {
		return nil, fmt.Errorf("execution failed: %w check log at %q", err, log.Name())
	}

	return args, nil
}

// parseColorInfo extracts detailed color information about the video stream of an input file using ffprobe.
// It constructs and executes a command to extract specific metadata related to color spaces, primary colors, transfer characteristics, pixel formats, and other frame details.
// The function returns the parsed color information or an error if extraction fails.
func parseColorInfo(ctx context.Context, inputFile string) (libcodec.ColorInfo, error) {
	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-select_streams", "v:0",
		"-analyzeduration", "6000M",
		"-probesize", "6000M",
		"-print_format", "json",
		"-show_frames",
		"-read_intervals", "%+#1",
		"-show_entries", "frame=color_space,color_primaries,color_transfer,side_data_list,pix_fmt",
		"-i", inputFile,
	}
	logger.Infof("parsing color info, calling ffprobe with args: %#v", args)
	cmd := exec.CommandContext(ctx, ffprobebinary, args...)
	o, err := cmd.Output()
	if err != nil && cmd.ProcessState.ExitCode() != 0 {
		return libcodec.ColorInfo{}, fmt.Errorf("failed to extract color information err: %v output: %s exit code: %d", err, o, cmd.ProcessState.ExitCode())
	}

	var ci ColorInfoWrapper
	if err := json.Unmarshal(o, &ci); err != nil {
		return libcodec.ColorInfo{}, fmt.Errorf("failed to unmarshal color info %q from ffprobe: %w", o, err)
	}

	return ci.Frames[0], nil
}

// buildCodec generates command line arguments for ffmpeg based on the specified codec type and CRF value, with optional color metadata.
// It supports various codecs including libx265, libsvtav1, hevc_nvenc, and can handle specific configurations based on the provided CRF value and color metadata.
// The function will always return a valid set of ffmpeg flags for a given codec, if an unrecognized codec is passed then a default libx265 arg slice will be built and returned.
func buildCodec(codec string, crf int, colorMeta libcodec.ColorInfo) []string {

	hevc_nvec := []string{
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

	switch strings.ToLower(codec) {
	case "copy":
		return []string{"-c:v", "copy"}
	case "libsvtav1":
		return libcodec.BuildLibSvtAv1("none", crf, colorMeta)
	case "hevc_nvenc":
		return hevc_nvec
	case "libx265_animation":
		return libcodec.BuildLibx265("animation", crf, colorMeta)
	case "libx265_grain":
		return libcodec.BuildLibx265("grain", crf, colorMeta)
	case "libx265":
		return libcodec.BuildLibx265("none", crf, colorMeta)
	default:
		return libcodec.BuildLibx265("none", crf, colorMeta)
	}
}
