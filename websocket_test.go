package main

import (
	"os"
	"testing"
)

const (
	gettysburg     = "Four score and seven years ago our fathers brought forth on this continent, a new nation, conceived in Liberty, and dedicated to the proposition that all men are created equal.    Now we are engaged in a great civil war, testing whether that nation, or any nation so conceived and so dedicated, can long endure. We are met on a great battle-field of that war. We have come to dedicate a portion of that field, as a final resting place for those who here gave their lives that that nation might live. It is altogether fitting and proper that we should do this.    But, in a larger sense, we can not dedicate—we can not consecrate—we can not hallow—this ground. The brave men, living and dead, who struggled here, have consecrated it, far above our poor power to add or detract. The world will little note, nor long remember what we say here, but it can never forget what they did here. It is for us the living, rather, to be dedicated here to the unfinished work which they who fought here have thus far so nobly advanced. It is rather for us to be here dedicated to the great task remaining before us—that from these honored dead we take increased devotion to that cause for which they gave the last full measure of devotion—that we here highly resolve that these dead shall not have died in vain—that this nation, under God, shall have a new birth of freedom—and that government of the people, by the people, for the people, shall not perish from the earth."
	gettysburgTail = "here gave their lives that that nation might live. It is altogether fitting and proper that we should do this.    But, in a larger sense, we can not dedicate—we can not consecrate—we can not hallow—this ground. The brave men, living and dead, who struggled here, have consecrated it, far above our poor power to add or detract. The world will little note, nor long remember what we say here, but it can never forget what they did here. It is for us the living, rather, to be dedicated here to the unfinished work which they who fought here have thus far so nobly advanced. It is rather for us to be here dedicated to the great task remaining before us—that from these honored dead we take increased devotion to that cause for which they gave the last full measure of devotion—that we here highly resolve that these dead shall not have died in vain—that this nation, under God, shall have a new birth of freedom—and that government of the people, by the people, for the people, shall not perish from the earth."
)

func tempWraper(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp("", "tailLogTest*.txt")
	if err != nil {
		t.Fatalf("couldn't create temp file: %v", err)
	}
	return f
}

func openWrap(t *testing.T, f string) *os.File {
	t.Helper()
	file, err := os.Open(f)
	if err != nil {
		t.Fatalf("couldn't open file: %v", err)
	}
	return file
}

func TestTailLog(t *testing.T) {
	tt := []struct {
		desc        string
		file        *os.File
		content     string
		expected    string
		shouldError bool
		clobber     bool
	}{
		{
			desc:        "Good File",
			file:        tempWraper(t),
			content:     "more tests are coming\nin a follow up commit",
			expected:    "in a follow up commit",
			shouldError: false,
			clobber:     true,
		},
		{
			desc:        "Empty File",
			file:        tempWraper(t),
			content:     "",
			expected:    "",
			shouldError: false,
			clobber:     true,
		},
		{
			desc:        "one short line",
			file:        tempWraper(t),
			content:     "this file has one line",
			expected:    "this file has one line",
			shouldError: false,
			clobber:     true,
		},
		{
			desc:        "one long line",
			file:        tempWraper(t),
			content:     gettysburg,
			expected:    gettysburgTail,
			shouldError: false,
			clobber:     true,
		},
		{
			desc:        "carriage returns are annoying",
			file:        tempWraper(t),
			content:     "ffmpeg streams output\r with carriage returns which is weird\r when you redirect output\r",
			expected:    "when you redirect output",
			shouldError: false,
			clobber:     true,
		},
		{
			desc:        "actual ffmpeg log",
			file:        openWrap(t, "ffmpeg_test.log"),
			content:     "",
			expected:    "frame= 1077 fps= 35 q=17.0 size=   30720KiB time=00:00:44.87 bitrate=5607.6kbits/s speed=1.45x",
			shouldError: false,
			clobber:     false,
		},
	}
	for _, tc := range tt {
		if tc.clobber {
			defer os.Remove(tc.file.Name())
			_, err := tc.file.WriteString(tc.content)
			if err != nil {
				t.Fatalf("%s: couldn't populate temp file: %v", tc.desc, err)
			}
			if err := tc.file.Sync(); err != nil {
				t.Fatalf("%s: couldn't sync temp file: %v", tc.desc, err)
			}
		}
		c, err := tailLog(tc.file.Name())
		if (err != nil && !tc.shouldError) || (err == nil && tc.shouldError) {
			t.Errorf("%s: unexpected error state: %v", tc.desc, err)
		}
		if c != tc.expected {
			t.Errorf("%s: got %q, want: %q", tc.desc, c, tc.expected)
		}
	}
}
