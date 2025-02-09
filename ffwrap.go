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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/logger"
)

type colorCoords struct {
	Coordinates string
}

type colorInfoWrapper struct {
	Frames []colorInfo `json:"frames"`
}

type colorInfo struct {
	Color_space     string          `json:"color_space"`
	Color_primaries string          `json:"color_primaries"`
	Color_transfer  string          `json:"color_transfer"`
	Side_data_list  []colorSideInfo `json:"side_data_list"`
}

type colorSideInfo struct {
	Side_data_type string `json:"side_data_type"`
	Red_x          string `json:"red_x"`
	Red_y          string `json:"red_y"`
	Green_x        string `json:"green_x"`
	Green_y        string `json:"green_y"`
	Blue_x         string `json:"Blue_x"`
	Blue_y         string `json:"Blue_y"`
	White_point_x  string `json:"White_point_x"`
	White_point_y  string `json:"White_point_y"`
	Min_luminance  string `json:"Min_luminance"`
	Max_luminance  string `json:"Max_luminance"`
	Max_content    int    `json:"Max_content"`
	Max_average    int    `json:"Max_average"`
}

type FfprobeOutput struct {
	Streams []FfprobeStreams
	Format  FfprobeFormat
}

type FfprobeStreams struct {
	Codec      string `json:"codec_name"`
	Codec_type string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type FfprobeFormat struct {
	Duration string `json:"duration"`
}

var (
	// cdregex extracts the correct crop filter from an ffmpeg cropdetect run
	cdregex  = regexp.MustCompile(`t:([\d]*).*?(crop=[-\d:]*)`)
	ffquiet  = []string{"-y", "-hide_banner", "-stats", "-loglevel", "error"}
	ffcommon = []string{"-probesize", "6000M", "-analyzeduration", "6000M"}
)

const (
	side_data_type_mastering   = "Mastering display metadata"
	side_data_type_light_level = "Content light level metadata"
)

// detectCrop uses the FFmpeg tool to automatically detect and return the crop filter string for an input video file.
// It constructs a command with arguments suitable for the FFmpeg cropdetect filter, runs the command, and processes its output
// to extract and return the detected crop parameters as a string. If any error occurs during this process, it returns an error message.
func detectCrop(inputFile string) (string, error) {
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
func probeMetadata(source string) (MediaMetadata, error) {
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
func ffmpegTranscode(tj TranscodeJob) ([]string, error) {
	args := append(ffquiet, ffcommon...)

	args = append(args, "-i", tj.JobDefinition.Source)

	mapargs := []string{
		"-map", "0:v:0",
		"-map", "0:a:m:language:eng:?",
		"-map", "0:s:m:language:eng:?",
		"-map", "0:t:?"}

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

	colorMeta, err := parseColorInfo(tj.JobDefinition.Source)
	if err != nil {
		logger.Errorf("failed to parse color metadata: %v", err)
	}
	logger.Infof("got color metadata: %#v", colorMeta)
	args = append(args, buildCodec(tj.JobDefinition.Codec, tj.JobDefinition.Crf, colorMeta)...)
	args = append(args, "-c:a", "copy", "-c:s", "copy", "-c:t", "copy")
	args = append(args, mapargs...)
	args = append(args, tj.JobDefinition.Destination)

	log, err := registerLogFile(tj)
	if err != nil {
		logger.Errorf("failed to register log file: %v", err)
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
		return nil, fmt.Errorf("execution failed: %q check log at %q", err, log.Name())
	}

	return args, nil
}

// parseColorInfo extracts detailed color information about the video stream of an input file using ffprobe.
// It constructs and executes a command to extract specific metadata related to color spaces, primary colors, transfer characteristics, pixel formats, and other frame details.
// The function returns the parsed color information or an error if extraction fails.
func parseColorInfo(inputFile string) (colorInfo, error) {
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
		return colorInfo{}, fmt.Errorf("failed to extract color information err: %v output: %s exit code: %d", err, o, cmd.ProcessState.ExitCode())
	}

	var ci colorInfoWrapper
	if err := json.Unmarshal(o, &ci); err != nil {
		return colorInfo{}, fmt.Errorf("failed to unmarshal color info %q from ffprobe: %w", o, err)
	}

	return ci.Frames[0], nil
}

// evalColorCoordinateAv1 evaluates the color coordinate from a fraction string representation.
// It splits the input string by the '/' character to separate the numerator and denominator, converts them to float64 values,
// and returns their division as a float64 representing the color coordinate. If the input string is invalid or there's an error during conversion,
// it returns an error.
func evalColorCoordinateAv1(colorFrac string) (float64, error) {
	splits := strings.Split(colorFrac, "/")
	if len(splits) != 2 {
		return 0, fmt.Errorf("invalid color fraction: %s", colorFrac)
	}
	n, err := strconv.ParseFloat(splits[0], 64)
	if err != nil {
		return 0, err
	}

	d, err := strconv.ParseFloat(splits[1], 64)
	if err != nil {
		return 0, err
	}

	return n / d, nil
}

// evalColorCoordinate265 evaluates the color coordinate for libx265 from a fraction string representation.
// It splits the input string by the '/' character to separate the numerator and denominator, converts them to integer values,
// and returns their division multiplied by 50000 as an integer representing the color coordinate. If the input string is invalid or there's an error during conversion,
// it returns an error.
func evalColorCoordinate265(colorFrac string) (int, error) {
	splits := strings.Split(colorFrac, "/")
	if len(splits) != 2 {
		return 0, fmt.Errorf("invalid color fraction: %s", colorFrac)
	}
	n, err := strconv.Atoi(splits[0])
	if err != nil {
		return 0, err
	}

	d, err := strconv.Atoi(splits[1])
	if err != nil {
		return 0, err
	}

	return n * (50000 / d), nil
}

// evalLumCoordinate265 evaluates the luminance coordinate from a fraction string representation for use by libx265.
// It splits the input string by the '/' character to separate the numerator and denominator, converts them to integer values,
// and returns their division multiplied by 10000 as an integer representing the luminance coordinate. If the input string is invalid or there's an error during conversion,
// it returns an error.
func evalLumCoordinate265(colorFrac string) (int, error) {
	splits := strings.Split(colorFrac, "/")
	if len(splits) != 2 {
		return 0, fmt.Errorf("invalid luminance fraction: %s", colorFrac)
	}
	n, err := strconv.Atoi(splits[0])
	if err != nil {
		return 0, err
	}

	d, err := strconv.Atoi(splits[1])
	if err != nil {
		return 0, err
	}

	return n * (10000 / d), nil
}

// parseColorCoordsAv1 takese Color Side Info pulled through ffprobe and returns
// color coordinatues usable by libsvtav1 or an error.
func parseColorCoordsAv1(csi colorSideInfo) (colorCoords, error) {
	rx, err := evalColorCoordinateAv1(csi.Red_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval red x: %v", err)
	}
	ry, err := evalColorCoordinateAv1(csi.Red_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval red y: %v", err)
	}
	r := fmt.Sprintf("R(%f,%f)", rx, ry)
	gx, err := evalColorCoordinateAv1(csi.Green_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval green x: %v", err)
	}
	gy, err := evalColorCoordinateAv1(csi.Green_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval green y: %v", err)
	}
	g := fmt.Sprintf("G(%f,%f)", gx, gy)
	bx, err := evalColorCoordinateAv1(csi.Blue_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval blue x: %v", err)
	}
	by, err := evalColorCoordinateAv1(csi.Blue_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval blue y: %v", err)
	}
	b := fmt.Sprintf("B(%f,%f)", bx, by)
	wx, err := evalColorCoordinateAv1(csi.White_point_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval wpx: %v", err)
	}
	wy, err := evalColorCoordinateAv1(csi.White_point_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval wpy: %v", err)
	}
	wp := fmt.Sprintf("WP(%f,%f)", wx, wy)
	maxl, err := evalColorCoordinateAv1(csi.Max_luminance)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval maxl: %v", err)
	}
	minl, err := evalColorCoordinateAv1(csi.Min_luminance)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval minl: %v", err)
	}
	lm := fmt.Sprintf("L(%f,%f)", maxl, minl)

	return colorCoords{g + b + r + wp + lm}, nil
}

// parseColorCoordsAv1 takese Color Side Info pulled through ffprobe and returns
// color coordinatues usable by libx265 or an error.
func parseColorCoords265(csi colorSideInfo) (colorCoords, error) {
	rx, err := evalColorCoordinate265(csi.Red_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval red x: %v", err)
	}
	ry, err := evalColorCoordinate265(csi.Red_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval red y: %v", err)
	}
	r := fmt.Sprintf("R(%d,%d)", rx, ry)
	gx, err := evalColorCoordinate265(csi.Green_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval green x: %v", err)
	}
	gy, err := evalColorCoordinate265(csi.Green_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval green y: %v", err)
	}
	g := fmt.Sprintf("G(%d,%d)", gx, gy)
	bx, err := evalColorCoordinate265(csi.Blue_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval blue x: %v", err)
	}
	by, err := evalColorCoordinate265(csi.Blue_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval blue y: %v", err)
	}
	b := fmt.Sprintf("B(%d,%d)", bx, by)
	wx, err := evalColorCoordinate265(csi.White_point_x)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval wpx: %v", err)
	}
	wy, err := evalColorCoordinate265(csi.White_point_y)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval wpy: %v", err)
	}
	wp := fmt.Sprintf("WP(%d,%d)", wx, wy)
	maxl, err := evalLumCoordinate265(csi.Max_luminance)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval maxl: %v", err)
	}
	minl, err := evalLumCoordinate265(csi.Min_luminance)
	if err != nil {
		return colorCoords{}, fmt.Errorf("failed to eval minl: %v", err)
	}
	lm := fmt.Sprintf("L(%d,%d)", maxl, minl)

	return colorCoords{g + b + r + wp + lm}, nil
}

// buildCodec generates command line arguments for ffmpeg based on the specified codec type and CRF value, with optional color metadata.
// It supports various codecs including libx265, libsvtav1, hevc_nvenc, and can handle specific configurations based on the provided CRF value and color metadata.
// The function will always return a valid set of ffmpeg flags for a given codec, if an unrecognized codec is passed then a default libx265 arg slice will be built and returned.
func buildCodec(codec string, crf int, colorMeta colorInfo) []string {
	libx265 := []string{
		"-c:v", "libx265",
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", "medium",
		"-profile:v", "main10",
	}

	libsvtav1 := []string{
		"-c:v", "libsvtav1",
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", "6",
	}

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
		svtav1Params := []string{"tune=0:enable-overlays=1:input-depth=10"}
		if colorMeta.Color_space != "" {
			libsvtav1 = append(libsvtav1, "-colorspace", colorMeta.Color_space)
		}
		if colorMeta.Color_primaries != "" {
			libsvtav1 = append(libsvtav1, "-color_primaries:v", colorMeta.Color_primaries)
		}
		if colorMeta.Color_transfer != "" {
			libsvtav1 = append(libsvtav1, "-color_trc:v", colorMeta.Color_transfer)
		}
		for _, sd := range colorMeta.Side_data_list {
			switch strings.ToLower(sd.Side_data_type) {
			case strings.ToLower(side_data_type_mastering):
				cc, err := parseColorCoordsAv1(sd)
				if err != nil {
					logger.Errorf("failed to parse color coordinates: %v", err)
					continue
				}
				svtav1Params = append(svtav1Params, "enable-hdr=1")
				svtav1Params = append(svtav1Params, fmt.Sprintf("mastering-display=%s", cc.Coordinates))
			case strings.ToLower(side_data_type_light_level):
				svtav1Params = append(svtav1Params, fmt.Sprintf("content-light=%d,%d", sd.Max_content, sd.Max_average))
			}
			svtav1Params = append(svtav1Params, "chroma-sample-position=topleft")
		}
		libsvtav1 = append(libsvtav1, "-svtav1-params", strings.Join(svtav1Params, ":"))
		return append(libsvtav1, "-pix_fmt", "yuv420p10le")

	case "hevc_nvenc":
		return hevc_nvec
	case "libx265_animation":
		libx265 = append(libx265, "-tune", "animation")
		x265params := []string{
			"hdr-opt=1",
			"repeat-headers=1",
		}
		hdrcolor, x265color, err := libx265HDR(colorMeta)
		if err != nil {
			logger.Errorf("failed to generate color args, continuing without: %v", err)
		}

		if len(hdrcolor) > 0 {
			libx265 = append(libx265, hdrcolor...)
		}
		if len(x265color) > 0 {
			x265params = append(x265params, x265color...)
			libx265 = append(libx265, "-x265-params", strings.Join(x265params, ":"))
		}
		return append(libx265, "-pix_fmt", "yuv420p10le")
	case "libx265_grain":
		libx265 = append(libx265, "-tune", "grain")
		x265params := []string{
			"hdr-opt=1",
			"repeat-headers=1",
		}
		hdrcolor, x265color, err := libx265HDR(colorMeta)
		if err != nil {
			logger.Errorf("failed to generate color args, continuing without: %v", err)
		}

		if len(hdrcolor) > 0 {
			libx265 = append(libx265, hdrcolor...)
		}
		if len(x265color) > 0 {
			x265params = append(x265params, x265color...)
			libx265 = append(libx265, "-x265-params", strings.Join(x265params, ":"))
		}
		return append(libx265, "-pix_fmt", "yuv420p10le")
	default:
		x265params := []string{
			"hdr-opt=1",
			"repeat-headers=1",
		}
		hdrcolor, x265color, err := libx265HDR(colorMeta)
		if err != nil {
			logger.Errorf("failed to generate color args, continuing without: %v", err)
		}

		if len(hdrcolor) > 0 {
			libx265 = append(libx265, hdrcolor...)
		}
		if len(x265color) > 0 {
			x265params = append(x265params, x265color...)
			libx265 = append(libx265, "-x265-params", strings.Join(x265params, ":"))
		}
		return append(libx265, "-pix_fmt", "yuv420p10le")
	}
}

// libx265HDR processes color metadata for use with the libx265 codec to enable High-Dynamic Range (HDR) settings.
// It takes a `colorInfo` struct as input, which contains information about the color space, primaries, and transfer characteristics,
// along with side data list containing mastering or light level data. The function returns the processed libx265 and x265params slices
// that can be used to configure the encoding process for HDR support in the libx265 codec. If any error occurs during processing, it is returned.
func libx265HDR(colorMeta colorInfo) (libx265, x265params []string, err error) {
	if colorMeta.Color_space != "" {
		libx265 = append([]string{"-colorspace", colorMeta.Color_space}, libx265...)
		x265params = append(x265params, fmt.Sprintf("colormatrix=%s", colorMeta.Color_space))
	}
	if colorMeta.Color_primaries != "" {
		libx265 = append([]string{"-color_primaries:v", colorMeta.Color_primaries}, libx265...)
		x265params = append(x265params, fmt.Sprintf("colorprim=%s", colorMeta.Color_primaries))
	}
	if colorMeta.Color_transfer != "" {
		libx265 = append([]string{"-color_trc:v", colorMeta.Color_transfer}, libx265...)
		x265params = append(x265params, fmt.Sprintf("transfer=%s", colorMeta.Color_transfer))
	}
	for _, sd := range colorMeta.Side_data_list {
		switch strings.ToLower(sd.Side_data_type) {
		case strings.ToLower(side_data_type_mastering):
			cc, err := parseColorCoords265(sd)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse color coordinates: %v", err)
			}
			x265params = append(x265params, fmt.Sprintf("master-display=%s", cc.Coordinates))
		case strings.ToLower(side_data_type_light_level):
			x265params = append(x265params, fmt.Sprintf("content-light=%d,%d", sd.Max_content, sd.Max_average))
		}
	}
	return libx265, x265params, nil
}
