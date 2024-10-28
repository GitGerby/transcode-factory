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
	f, err := os.CreateTemp("", "tailLogTest*.txt")
	if err != nil {
		t.Fatalf("couldn't create temp file: %v", err)
	}
	return f
}

func TestTailLog(t *testing.T) {
	tt := []struct {
		desc        string
		file        *os.File
		content     string
		expected    string
		shouldError bool
	}{
		{
			desc:        "Good File",
			file:        tempWraper(t),
			content:     "more tests are coming\nin a follow up commit",
			expected:    "in a follow up commit",
			shouldError: false,
		},
		{
			desc:        "Empty File",
			file:        tempWraper(t),
			content:     "",
			expected:    "",
			shouldError: false,
		},
		{
			desc:        "one short line",
			file:        tempWraper(t),
			content:     "this file has one line",
			expected:    "this file has one line",
			shouldError: false,
		},
		{
			desc:        "one long line",
			file:        tempWraper(t),
			content:     gettysburg,
			expected:    gettysburgTail,
			shouldError: false,
		},
	}
	for _, tc := range tt {

		defer os.Remove(tc.file.Name())
		_, err := tc.file.WriteString(tc.content)
		tc.file.Sync()
		if err != nil {
			t.Fatalf("%s: couldn't populate temp file: %v", tc.desc, err)
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
