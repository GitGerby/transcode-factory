package main

import (
	"testing"
)

func TestParseColorCoordsAv1(t *testing.T) {
	// Test case 1: Valid color side information
	csi := colorSideInfo{
		Red_x:         "30/100",
		Red_y:         "30/100",
		Green_x:       "40/100",
		Green_y:       "40/100",
		Blue_x:        "20/100",
		Blue_y:        "20/100",
		White_point_x: "90/100",
		White_point_y: "100/100",
		Max_luminance: "100/100",
		Min_luminance: "0/100",
	}
	expected := "G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000)"
	result, err := parseColorCoordsAv1(csi)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Coordinates != expected {
		t.Errorf("Incorrect color coordinates: expected %s, got %s", expected, result.Coordinates)
	}

	// Test case 2: Invalid color side information (missing values)
	csi = colorSideInfo{
		Red_x:         "30/100",
		Red_y:         "30/100",
		Green_x:       "40/100",
		White_point_x: "90/100",
		White_point_y: "100/100",
	}
	_, err = parseColorCoordsAv1(csi)
	if err == nil {
		t.Errorf("Expected error due to missing values")
	}

	// Test case 3: Invalid color side information (not a number)
	csi = colorSideInfo{
		Red_x:         "30/100",
		Red_y:         "30/100",
		Green_x:       "40/100",
		Green_y:       "40/100",
		Blue_x:        "20/100",
		Blue_y:        "20/100",
		White_point_x: "90/100",
		White_point_y: "100/100",
		Max_luminance: "a/100",
		Min_luminance: "0/100",
	}
	_, err = parseColorCoordsAv1(csi)
	if err == nil {
		t.Errorf("Expected error due to not a number")
	}
}

func TestParseColorCoords265(t *testing.T) {
	// Test case 1: Valid color side information
	csi := colorSideInfo{
		Red_x:         "30/100",
		Red_y:         "30/100",
		Green_x:       "40/100",
		Green_y:       "40/100",
		Blue_x:        "20/100",
		Blue_y:        "20/100",
		White_point_x: "90/100",
		White_point_y: "100/100",
		Max_luminance: "100/100",
		Min_luminance: "0/100",
	}
	expected := "G(20000,20000)B(10000,10000)R(15000,15000)WP(45000,50000)L(10000,0)"
	result, err := parseColorCoords265(csi)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Coordinates != expected {
		t.Errorf("Incorrect color coordinates: expected %s, got %s", expected, result.Coordinates)
	}

	// Test case 2: Invalid color side information (missing values)
	csi = colorSideInfo{
		Red_x:         "30/100",
		Red_y:         "30/100",
		Green_x:       "40/100",
		White_point_x: "90/100",
		White_point_y: "100/100",
	}
	_, err = parseColorCoordsAv1(csi)
	if err == nil {
		t.Errorf("Expected error due to missing values")
	}

	// Test case 3: Invalid color side information (not a number)
	csi = colorSideInfo{
		Red_x:         "30/100",
		Red_y:         "30/100",
		Green_x:       "40/100",
		Green_y:       "40/100",
		Blue_x:        "20/100",
		Blue_y:        "20/100",
		White_point_x: "90/100",
		White_point_y: "100/100",
		Max_luminance: "a/100",
		Min_luminance: "0/100",
	}
	_, err = parseColorCoordsAv1(csi)
	if err == nil {
		t.Errorf("Expected error due to not a number")
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
			desc: "Test case 1: Valid color meta data",
			colorMeta: colorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []colorSideInfo{
					{
						Side_data_type: side_data_type_mastering,
						Red_x:          "1/3",
						Red_y:          "2/3",
						Green_x:        "1/4",
						Green_y:        "3/4",
						Blue_x:         "1/5",
						Blue_y:         "4/5",
						White_point_x:  "1/2",
						White_point_y:  "2/3",
						Max_luminance:  "1000/1",
						Min_luminance:  "10/1",
					},
					{
						Side_data_type: side_data_type_light_level,
						Max_content:    1000000,
						Max_average:    50000,
					},
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
				"master-display=G(12500,37500)B(10000,40000)R(16666,33332)WP(25000,33332)L(10000000,100000)",
				"content-light=1000000,50000",
			},
			shouldError: false,
		},
		{
			desc: "Test case 2: invalid color meta data",
			colorMeta: colorInfo{
				Color_space:     "bt709",
				Color_primaries: "bt709",
				Color_transfer:  "srgb",
				Side_data_list: []colorSideInfo{
					{
						Side_data_type: side_data_type_mastering,
						Red_x:          "1/a",
						Red_y:          "2/3",
						Green_x:        "1/4",
						Green_y:        "3/4",
						Blue_x:         "1/5",
						Blue_y:         "4/5",
						White_point_x:  "1/2",
						White_point_y:  "2/3",
						Max_luminance:  "1000/1",
						Min_luminance:  "10/1",
					},
					{
						Side_data_type: side_data_type_light_level,
						Max_content:    1000000,
						Max_average:    50000,
					},
				},
			},
			expectedLib:   nil,
			expectedParam: nil,
			shouldError:   true,
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
