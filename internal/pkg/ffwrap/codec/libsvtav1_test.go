package codec

import (
	"testing"
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
			csi:         ValidMasteringColorInfo,
			expected:    "G(0.400000,0.400000)B(0.200000,0.200000)R(0.300000,0.300000)WP(0.900000,1.000000)L(1.000000,0.000000)",
			shouldError: false,
		},
		{
			desc:        "Missing color side information",
			csi:         MissingMasteringColorInfo,
			expected:    "",
			shouldError: true,
		},
		{
			desc:        "Not a number color side information",
			csi:         NanMasteringColorInfo,
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
