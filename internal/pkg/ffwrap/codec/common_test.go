package codec

import (
	"testing"
)

// Vars used for tests
var (
	validColorMetaData = ColorInfo{
		Color_space:     "bt709",
		Color_primaries: "bt709",
		Color_transfer:  "bt709",
		Side_data_list:  []ColorSideInfo{validLightColorInfo, validMasteringColorInfo}}

	validMasteringColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeMastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		Green_y:        "40/100",
		Blue_x:         "20/100",
		Blue_y:         "20/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
		Max_luminance:  "100/100",
		Min_luminance:  "0/100",
	}
	missingMasteringColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeMastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
	}
	nanMasteringColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeMastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		Green_y:        "40/100",
		Blue_x:         "a/100",
		Blue_y:         "20/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
		Max_luminance:  "a/100",
		Min_luminance:  "0/100",
	}
	validLightColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeLightLevel,
		Max_content:    700,
		Max_average:    200,
	}
	missingLightColorInfo = ColorSideInfo{
		Side_data_type: SideDataTypeLightLevel,
		Max_content:    700,
	}
)

// Define a struct for each test case, including input and expected output
type buildCodecTestCase struct {
	desc      string
	codec     string
	crf       int
	colorMeta ColorInfo
	expected  []string
}

func TestBuildCodec(t *testing.T) {
	// Define the test cases
	testCases := []buildCodecTestCase{
		{
			desc:     "copy -- good",
			codec:    "copy",
			expected: []string{"-c:v", "copy"},
		},
		{
			desc:  "hevc_nvenc -- sdr",
			codec: "hevc_nvenc",
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []ColorSideInfo{},
			},
			expected: []string{
				"-pix_fmt", "p010le",
				"-c:v", "hevc_nvenc",
				"-rc", "1",
				"-cq", "0",
				"-profile:v", "1",
				"-tier", "1",
				"-spatial_aq", "1",
				"-temporal_aq", "1",
				"-preset", "1",
				"-b_ref_mode", "2",
			},
		},
		{
			// note that the current implementation of nvenc here does not support hdr
			desc:  "hevc_nvenc -- hdr",
			codec: "hevc_nvenc",
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
				},
			},
			expected: []string{
				"-pix_fmt", "p010le",
				"-c:v", "hevc_nvenc",
				"-rc", "1",
				"-cq", "0",
				"-profile:v", "1",
				"-tier", "1",
				"-spatial_aq", "1",
				"-temporal_aq", "1",
				"-preset", "1",
				"-b_ref_mode", "2",
			},
		},
		{
			desc:  "libx265 -- good HDR",
			codec: "libx265",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
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
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []ColorSideInfo{},
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
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					nanMasteringColorInfo,
					validLightColorInfo,
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
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
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
			desc:  "libx265_grain -- good",
			codec: "libx265_grain",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libx265",
				"-crf", "23",
				"-preset", "medium",
				"-profile:v", "main10",
				"-tune", "grain",
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
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
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
			desc:  "libsvtav1 -- good HDR -- grain low",
			codec: "libsvtav1_grain:low",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft:content-light=700,200:chroma-sample-position=topleft:film-grain=5",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- good HDR -- grain medium",
			codec: "libsvtav1_grain:medium",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft:content-light=700,200:chroma-sample-position=topleft:film-grain=8",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- good HDR -- grain high",
			codec: "libsvtav1_grain:high",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
				},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:enable-hdr=1:mastering-display=G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000):chroma-sample-position=topleft:content-light=700,200:chroma-sample-position=topleft:film-grain=12",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- NANMasteringColor HDR",
			codec: "libsvtav1",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					nanMasteringColorInfo,
					validLightColorInfo,
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
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []ColorSideInfo{},
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
			desc:  "libsvtav1 -- Good SDR -- grain low",
			codec: "libsvtav1_grain:low",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []ColorSideInfo{},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:film-grain=5",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- Good SDR -- grain medium",
			codec: "libsvtav1_grain:medium",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []ColorSideInfo{},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:film-grain=8",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "libsvtav1 -- Good SDR -- grain high",
			codec: "libsvtav1_grain:high",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list:  []ColorSideInfo{},
			},
			expected: []string{
				"-c:v", "libsvtav1",
				"-crf", "23",
				"-preset", "6",
				"-colorspace", "bt709",
				"-color_primaries:v", "bt709",
				"-color_trc:v", "srgb",
				"-svtav1-params", "tune=0:enable-overlays=1:input-depth=10:film-grain=12",
				"-pix_fmt", "yuv420p10le",
			},
		},
		{
			desc:  "default case, unknown codec",
			codec: "this is never going to resolve to a real codec I hope",
			crf:   23,
			colorMeta: ColorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []ColorSideInfo{
					validMasteringColorInfo,
					validLightColorInfo,
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
	}

	// Loop over the test cases and run the function under test
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			result := BuildCodec(tc.codec, tc.crf, tc.colorMeta)
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
