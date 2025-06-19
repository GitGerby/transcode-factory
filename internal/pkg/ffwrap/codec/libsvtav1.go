package codec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/logger"
)

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

// parseColorCoordsAv1 takese Color Side Info pulled through ffprobe and returns
// color coordinatues usable by libsvtav1 or an error.
func parseColorCoordsAv1(csi ColorSideInfo) (ColorCoords, error) {
	rx, err := evalColorCoordinateAv1(csi.Red_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval red x: %v", err)
	}
	ry, err := evalColorCoordinateAv1(csi.Red_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval red y: %v", err)
	}
	r := fmt.Sprintf("R(%f,%f)", rx, ry)
	gx, err := evalColorCoordinateAv1(csi.Green_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval green x: %v", err)
	}
	gy, err := evalColorCoordinateAv1(csi.Green_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval green y: %v", err)
	}
	g := fmt.Sprintf("G(%f,%f)", gx, gy)
	bx, err := evalColorCoordinateAv1(csi.Blue_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval blue x: %v", err)
	}
	by, err := evalColorCoordinateAv1(csi.Blue_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval blue y: %v", err)
	}
	b := fmt.Sprintf("B(%f,%f)", bx, by)
	wx, err := evalColorCoordinateAv1(csi.White_point_x)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval wpx: %v", err)
	}
	wy, err := evalColorCoordinateAv1(csi.White_point_y)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval wpy: %v", err)
	}
	wp := fmt.Sprintf("WP(%f,%f)", wx, wy)
	maxl, err := evalColorCoordinateAv1(csi.Max_luminance)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval maxl: %v", err)
	}
	minl, err := evalColorCoordinateAv1(csi.Min_luminance)
	if err != nil {
		return ColorCoords{}, fmt.Errorf("failed to eval minl: %v", err)
	}
	lm := fmt.Sprintf("L(%f,%f)", maxl, minl)

	return ColorCoords{g + b + r + wp + lm}, nil
}

func BuildLibSvtAv1(grain string, crf int, colorMeta ColorInfo) []string {
	libsvtav1 := []string{
		"-c:v", "libsvtav1",
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", "6",
	}

	switch grain {
	case "low":
		libsvtav1 = append(libsvtav1, "-svtav1-params", "film-grain=10")
	}

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
		case strings.ToLower(SideDataTypeMastering):
			cc, err := parseColorCoordsAv1(sd)
			if err != nil {
				logger.Errorf("failed to parse color coordinates: %v", err)
				continue
			}
			svtav1Params = append(svtav1Params, "enable-hdr=1")
			svtav1Params = append(svtav1Params, fmt.Sprintf("mastering-display=%s", cc.Coordinates))
		case strings.ToLower(SideDataTypeLightLevel):
			svtav1Params = append(svtav1Params, fmt.Sprintf("content-light=%d,%d", sd.Max_content, sd.Max_average))
		}
		svtav1Params = append(svtav1Params, "chroma-sample-position=topleft")
	}
	libsvtav1 = append(libsvtav1, "-svtav1-params", strings.Join(svtav1Params, ":"))
	return append(libsvtav1, "-pix_fmt", "yuv420p10le")
}
