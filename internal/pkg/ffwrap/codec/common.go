package codec

import "strings"

const (
	SideDataTypeMastering  = "Mastering display metadata"
	SideDataTypeLightLevel = "Content light level metadata"
)

type ColorCoords struct {
	Coordinates string
}

type ColorInfo struct {
	Color_space     string          `json:"color_space"`
	Color_primaries string          `json:"color_primaries"`
	Color_transfer  string          `json:"color_transfer"`
	Side_data_list  []ColorSideInfo `json:"side_data_list"`
}

type ColorSideInfo struct {
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

// BuildCodec generates command line arguments for ffmpeg based on the specified codec type and CRF value, with optional color metadata.
// It supports various codecs including libx265, libsvtav1, hevc_nvenc, and can handle specific configurations based on the provided CRF value and color metadata.
// The function will always return a valid set of ffmpeg flags for a given codec, if an unrecognized codec is passed then a default libx265 arg slice will be built and returned.
func BuildCodec(codec string, crf int, colorMeta ColorInfo) []string {
	switch strings.ToLower(codec) {
	case "copy":
		return []string{"-c:v", "copy"}
	case "hevc_nvenc":
		return buildNvencHevc(crf)
	case "libsvtav1":
		return buildLibSvtAv1("none", crf, colorMeta)
	case "libsvtav1_grain:low":
		return buildLibSvtAv1("low", crf, colorMeta)
	case "libsvtav1_grain:medium":
		return buildLibSvtAv1("medium", crf, colorMeta)
	case "libsvtav1_grain:high":
		return buildLibSvtAv1("high", crf, colorMeta)
	case "libx265_animation":
		return buildLibx265("animation", crf, colorMeta)
	case "libx265_grain":
		return buildLibx265("grain", crf, colorMeta)
	case "libx265":
		return buildLibx265("none", crf, colorMeta)
	case "av1_amf":
		return buildAv1Amf()
	default:
		return buildLibx265("none", crf, colorMeta)
	}
}
