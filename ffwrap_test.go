package main

import (
	"testing"
)

var (
	validMasteringColorInfo = colorSideInfo{
		Side_data_type: side_data_type_mastering,
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
	missingMasteringColorInfo = colorSideInfo{
		Side_data_type: side_data_type_mastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
	}
	nanMasteringColorInfo = colorSideInfo{
		Side_data_type: side_data_type_mastering,
		Red_x:          "30/100",
		Red_y:          "30/100",
		Green_x:        "40/100",
		Green_y:        "40/100",
		Blue_x:         "20/100",
		Blue_y:         "20/100",
		White_point_x:  "90/100",
		White_point_y:  "100/100",
		Max_luminance:  "a/100",
		Min_luminance:  "0/100",
	}
	validLightColorInfo = colorSideInfo{
		Side_data_type: side_data_type_light_level,
		Max_content:    700,
		Max_average:    200,
	}
	missingLightColorInfo = colorSideInfo{
		Side_data_type: side_data_type_light_level,
		Max_content:    700,
	}
)

func TestParseColorCoordsAv1(t *testing.T) {
	testCases := []struct {
		desc        string
		csi         colorSideInfo
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

func TestParseColorCoords265(t *testing.T) {
	testCases := []struct {
		desc        string
		csi         colorSideInfo
		expected    string
		shouldError bool
	}{
		{
			desc:        "Valid color side information",
			csi:         validMasteringColorInfo,
			expected:    "G(20000,20000)B(10000,10000)R(15000,15000)WP(45000,50000)L(10000,0)",
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
			result, err := parseColorCoords265(tc.csi)
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

func TestLibx265HDR(t *testing.T) {
	// Define the test cases using a struct with fields
	testCases := []struct {
		desc          string
		colorMeta     colorInfo
		expectedLib   []string
		expectedParam []string
		shouldError   bool
	}{
		{
			desc: "Test case 1: Valid color metadata",
			colorMeta: colorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []colorSideInfo{
					validLightColorInfo,
					validMasteringColorInfo,
				},
			},
			expectedLib: []string{
				"-color_trc:v", "srgb",
				"-color_primaries:v", "bt709",
				"-colorspace", "bt709",
			},
			expectedParam: []string{
				"colormatrix=bt709",
				"colorprim=bt709",
				"transfer=srgb",
				"content-light=700,200",
				"master-display=G(20000,20000)B(10000,10000)R(15000,15000)WP(45000,50000)L(10000,0)",
			},
			shouldError: false,
		},
		{
			desc: "Test case 2: invalid color metadata",
			colorMeta: colorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []colorSideInfo{
					nanMasteringColorInfo,
					validLightColorInfo,
				},
			},
			expectedLib:   nil,
			expectedParam: nil,
			shouldError:   true,
		},
		{
			desc: "Test case 3: missing light metadata",
			colorMeta: colorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []colorSideInfo{
					validMasteringColorInfo,
					missingLightColorInfo,
				},
			},
			expectedLib:   []string{"-color_trc:v", "srgb", "-color_primaries:v", "bt709", "-colorspace", "bt709"},
			expectedParam: []string{"colormatrix=bt709", "colorprim=bt709", "transfer=srgb", "master-display=G(20000,20000)B(10000,10000)R(15000,15000)WP(45000,50000)L(10000,0)", "content-light=700,0"},
			shouldError:   false,
		},
	}

	// Loop over the test cases and run the function under test
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			lib, param, err := libx265HDR(tc.colorMeta)
			if err != nil && !tc.shouldError {
				t.Errorf("Unexpected error: %v", err)
			}
			if err == nil && tc.shouldError {
				t.Errorf("Expected error but got none")
			}
			if len(lib) != len(tc.expectedLib) {
				t.Errorf("Expected output length to be %d, but got %d, libx265 result: %#v", len(tc.expectedLib), len(lib), lib)
			} else {
				for i := range lib {
					if lib[i] != tc.expectedLib[i] {
						t.Errorf("Expected item %d in output to be '%s', but was '%s'", i, tc.expectedLib[i], lib[i])
					}
				}
			}
			if len(param) != len(tc.expectedParam) {
				t.Errorf("Expected output length to be %d, but got %d, x265param result: %#v", len(tc.expectedParam), len(param), param)
			} else {
				for i := range param {
					if param[i] != tc.expectedParam[i] {
						t.Errorf("Expected item %d in output to be '%s', but was '%s'", i, tc.expectedParam[i], param[i])
					}
				}
			}
		})
	}
}
