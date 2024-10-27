package main

import (
	"os"
	"testing"
)

func TestTailLog(t *testing.T) {
	tt := []struct {
		desc        string
		content     string
		expected    string
		shouldError bool
	}{
		{
			desc:        "Good File",
			content:     "more tests are coming\nin a follow up commit",
			expected:    "in a follow up commit",
			shouldError: false,
		},
	}
	for _, tc := range tt {
		f, err := os.CreateTemp("", "tailLogTest*.txt")
		if err != nil {
			t.Fatalf("%q couldn't create temp file: %v", tc.desc, err)
		}
		defer os.Remove(f.Name())
		_, err = f.WriteString(tc.content)
		f.Sync()
		if err != nil {
			t.Fatalf("%s: couldn't populate temp file: %v", tc.desc, err)
		}
		c, err := tailLog(f.Name())
		if (err != nil && !tc.shouldError) || (err == nil && tc.shouldError) {
			t.Errorf("%s: unexpected error state: %v", tc.desc, err)
		}
		if c != tc.expected {
			t.Errorf("%s: got %q, want: %q", tc.desc, c, tc.expected)
		}
	}
}
