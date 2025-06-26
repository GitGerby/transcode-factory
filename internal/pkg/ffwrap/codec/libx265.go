package codec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/logger"
)

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

// ParseColorCoords265 takese Color Side Info pulled through ffprobe and returns
// color coordinatues usable by libx265 or an error.
func parseColorCoords265(csi ColorSideInfo) (ColorCoords, error) {
	rx, err := evalColorCoordinate265(csi.Red_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval red x: %v", err)
	}
	ry, err := evalColorCoordinate265(csi.Red_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval red y: %v", err)
	}
	r := fmt.Sprintf("R(%d,%d)", rx, ry)
	gx, err := evalColorCoordinate265(csi.Green_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval green x: %v", err)
	}
	gy, err := evalColorCoordinate265(csi.Green_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval green y: %v", err)
	}
	g := fmt.Sprintf("G(%d,%d)", gx, gy)
	bx, err := evalColorCoordinate265(csi.Blue_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval blue x: %v", err)
	}
	by, err := evalColorCoordinate265(csi.Blue_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval blue y: %v", err)
	}
	b := fmt.Sprintf("B(%d,%d)", bx, by)
	wx, err := evalColorCoordinate265(csi.White_point_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval wpx: %v", err)
	}
	wy, err := evalColorCoordinate265(csi.White_point_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval wpy: %v", err)
	}
	wp := fmt.Sprintf("WP(%d,%d)", wx, wy)
	maxl, err := evalLumCoordinate265(csi.Max_luminance)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval maxl: %v", err)
	}
	minl, err := evalLumCoordinate265(csi.Min_luminance)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval minl: %v", err)
	}
	lm := fmt.Sprintf("L(%d,%d)", maxl, minl)

	return ColorCoords{g + b + r + wp + lm}, nil
}

// libx265HDR processes color metadata for use with the libx265 codec to enable High-Dynamic Range (HDR) settings.
// It takes a `colorInfo` struct as input, which contains information about the color space, primaries, and transfer characteristics,
// along with side data list containing mastering or light level data. The function returns the processed libx265 and x265params slices
// that can be used to configure the encoding process for HDR support in the libx265 codec. If any error occurs during processing, it is returned.
func libx265HDR(colorMeta ColorInfo) (libx265, x265params []string, err error) {
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
		case strings.ToLower(SideDataTypeMastering):
			cc, err := parseColorCoords265(sd)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse color coordinates: %v", err)
			}
			x265params = append(x265params, fmt.Sprintf("master-display=%s", cc.Coordinates))
		case strings.ToLower(SideDataTypeLightLevel):
			x265params = append(x265params, fmt.Sprintf("content-light=%d,%d", sd.Max_content, sd.Max_average))
		}
	}
	return libx265, x265params, nil
}

func buildLibx265(tune string, crf int, colorMeta ColorInfo) []string {
	libx265 := []string{
		"-c:v", "libx265",
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", "medium",
		"-profile:v", "main10",
	}

	switch tune {
	case "animation":
		libx265 = append(libx265, "-tune", "animation")
	case "grain":
		libx265 = append(libx265, "-tune", "grain")
	}

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
