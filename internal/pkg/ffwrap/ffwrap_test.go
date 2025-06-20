package ffwrap

import (
	"testing"

	"github.com/gitgerby/transcode-factory/internal/pkg/ffwrap/codec"
)

// Define a struct for each test case, including input and expected output
type buildCodecTestCase struct {
	desc      string
	codec     string
	crf       int
	colorMeta codec.ColorInfo
	expected  []string
}

func TestBuildCodec(t *testing.T) {
	// Define the test cases
	testCases := []buildCodecTestCase{
		{
			desc:  "libx265 -- good HDR",
			codec: "libx265",
			crf:   23,
			colorMeta: codec.ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []codec.ColorSideInfo{
					codec.ValidMasteringColorInfo,
					codec.ValidLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libx265",
				"-crf", "23",
				"-preset", "medium",
				"-profile:v", "main10",
				"-color_trc:v", "srgb",
				"-color_primaries:v", "bt709",
				"-colorspace", "bt709",
				"-x265-params",
				"hdr-opt=1:repeat-headers=1:colormatrix=bt709:colorprim=bt709:transfer=srgb:master-display=G(20000,20000)B(10000,10000)R(15000,15000)WP(45000,50000)L(10000,0):content-light=700,200",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libx265 -- good sdr",
			codec: "libx265",
			crf:   23,
			colorMeta: codec.ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []codec.ColorSideInfo{},
			},
			expected: []string{
				"-c:v", "libx265",
				"-crf", "23",
				"-preset", "medium",
				"-profile:v", "main10",
				"-color_trc:v", "srgb",
				"-color_primaries:v", "bt709",
				"-colorspace", "bt709",
				"-x265-params", "hdr-opt=1:repeat-headers=1:colormatrix=bt709:colorprim=bt709:transfer=srgb",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libx265 -- NANMasteringColor HDR",
			codec: "libx265",
			crf:   23,
			colorMeta: codec.ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []codec.ColorSideInfo{
					codec.NanMasteringColorInfo,
					codec.ValidLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libx265",
				"-crf", "23",
				"-preset", "medium",
				"-profile:v", "main10",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libx265_animation -- good",
			codec: "libx265_animation",
			crf:   23,
			colorMeta: codec.ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []codec.ColorSideInfo{
					codec.ValidMasteringColorInfo,
					codec.ValidLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libx265",
				"-crf", "23",
				"-preset", "medium",
				"-profile:v", "main10",
				"-tune", "animation",
				"-color_trc:v", "srgb",
				"-color_primaries:v", "bt709",
				"-colorspace", "bt709",
				"-x265-params", "hdr-opt=1:repeat-headers=1:colormatrix=bt709:colorprim=bt709:transfer=srgb:master-display=G(20000,20000)B(10000,10000)R(15000,15000)WP(45000,50000)L(10000,0):content-light=700,200",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- good HDR",
			codec: "libsvtav1",
			crf:   23,
			colorMeta: codec.ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []codec.ColorSideInfo{
					codec.ValidMasteringColorInfo,
					codec.ValidLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft:content-light=700,200:chroma-sample-position=topleft",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- NANMasteringColor HDR",
			codec: "libsvtav1",
			crf:   23,
			colorMeta: codec.ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []codec.ColorSideInfo{
					codec.NanMasteringColorInfo,
					codec.ValidLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:content-light=700,200:chroma-sample-position=topleft",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- Good SDR",
			codec: "libsvtav1",
			crf:   23,
			colorMeta: codec.ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []codec.ColorSideInfo{},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:     "copy -- good",
			codec:    "copy",
			expected: []string{"-c:v", "copy"},
		},
	}

	// Loop over the test cases and run the function under test
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			result := buildCodec(tc.codec, tc.crf, tc.colorMeta)
			if len(result) != len(tc.expected) {
				t.Errorf("Expected length of output to be %d, but got %d, %#v", len(tc.expected), len(result), result)
			} else {
				for i := range result {
					if result[i] != tc.expected[i] {
						t.Errorf("Expected item %d in output to be '%s', but was '%s'", i, tc.expected[i], result[i])
					}
				}
			}
		})
	}
}
