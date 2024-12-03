//go:build linux || darwin
// +build linux darwin

package main

import (
	"os"
	"syscall"
	"testing"
)

func TestLowerPriority(t *testing.T) {
	oldPriority, err := syscall.Getpriority(syscall.PRIO_PROCESS, os.Getpid())
	if err != nil {
		t.Fatalf("failed to determine current priority of: %v", os.Getpid())
	}

	t.Cleanup(func() {
		syscall.Setpriority(syscall.PRIO_PROCESS, os.Getpid(), oldPriority)
	})

	err = lowerPriority()
	if err != nil {
		t.Errorf("failed to lower priority: %v", err)
	}
	newPriority, err := syscall.Getpriority(syscall.PRIO_PROCESS, os.Getpid())
	if err != nil {
		t.Fatalf("failed to determine new priority of: %v", os.Getpid())
	}
	// lowerPriority should have set the priority to 10
	if newPriority != 10 {
		t.Errorf("priority after lowering is %d, expected 10", newPriority)
	}
	// If the priority was unchanged, log that the test is indeterminate.
	if oldPriority == newPriority {
		t.Log("lowerPriority did not change the priority")
	}
	return nil
}
