package codec

import "strings"

// BuildCodec generates command line arguments for ffmpeg based on the specified codec type and CRF value, with optional color metadata.
// It supports various codecs including libx265, libsvtav1, hevc_nvenc, and can handle specific configurations based on the provided CRF value and color metadata.
// The function will always return a valid set of ffmpeg flags for a given codec, if an unrecognized codec is passed then a default libx265 arg slice will be built and returned.
func BuildCodec(codec string, crf int, colorMeta ColorInfo) []string {
	switch strings.ToLower(codec) {
	case "copy":
		return []string{"-c:v", "copy"}
	case "hevc_nvenc":
		return nvenc_hevc(crf)
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
	default:
		return buildLibx265("none", crf, colorMeta)
	}
}
