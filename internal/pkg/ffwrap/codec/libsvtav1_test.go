package codec

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseColorCoordsAv1(t *testing.T) {
	testCases := []struct {
		desc        string
		csi         ColorSideInfo
		expected    string
		shouldError bool
	}{
		{
			desc:        "Valid color side information",
			csi:         validMasteringColorInfo,
			expected:    "G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000)",
			shouldError: false,
		},
		{
			desc:        "Missing color side information",
			csi:         missingMasteringColorInfo,
			expected:    "",
			shouldError: true,
		},
		{
			desc:        "Not a number color side information",
			csi:         nanMasteringColorInfo,
			expected:    "",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			result, err := parseColorCoordsAv1(tc.csi)
			if err == nil && tc.shouldError {
				t.Errorf("%q: Expected error but got nil", tc.desc)
			}
			if err != nil && !tc.shouldError {
				t.Errorf("%q: got error: %v want: nil", tc.desc, err)
			}
			if result.Coordinates != tc.expected {
				t.Errorf("%q: unexpected result %v want: %v", tc.desc, result.Coordinates, tc.expected)
			}

		})
	}
}

func TestBuildLibSvtAv1(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc      string
		grain     string
		colorMeta ColorInfo
		expected  []string
	}{
		{
			desc:      "No grain",
			grain:     "none",
			colorMeta: validColorMetaData,
			expected:  []string{"-c:v", "libsvtav1", "-crf", "18", "-preset", "6", "-colorspace", "bt709", "-color_primaries:v", "bt709", "-color_trc:v", "bt709", "-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:content-light=700,200:chroma-sample-position=topleft:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft", "-pix_fmt", "yuv420p10le"},
		},
		{
			desc:      "low grain",
			grain:     "low",
			colorMeta: validColorMetaData,
			expected:  []string{"-c:v", "libsvtav1", "-crf", "18", "-preset", "6", "-colorspace", "bt709", "-color_primaries:v", "bt709", "-color_trc:v", "bt709", "-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:content-light=700,200:chroma-sample-position=topleft:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft:film-grain=5", "-pix_fmt", "yuv420p10le"},
		}, {
			desc:      "medium grain",
			grain:     "medium",
			colorMeta: validColorMetaData,
			expected:  []string{"-c:v", "libsvtav1", "-crf", "18", "-preset", "6", "-colorspace", "bt709", "-color_primaries:v", "bt709", "-color_trc:v", "bt709", "-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:content-light=700,200:chroma-sample-position=topleft:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft:film-grain=8", "-pix_fmt", "yuv420p10le"},
		},
		{
			desc:      "high grain",
			grain:     "high",
			colorMeta: validColorMetaData,
			expected:  []string{"-c:v", "libsvtav1", "-crf", "18", "-preset", "6", "-colorspace", "bt709", "-color_primaries:v", "bt709", "-color_trc:v", "bt709", "-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:content-light=700,200:chroma-sample-position=topleft:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft:film-grain=12", "-pix_fmt", "yuv420p10le"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			result := buildLibSvtAv1(tc.grain, 18, tc.colorMeta)
			if cmp.Diff(result, tc.expected) != "" {
				t.Errorf("unexpected result %#v want: %#v", result, tc.expected)
			}
		})
	}
}
