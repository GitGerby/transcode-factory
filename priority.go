//go:build linux || darwin
// +build linux darwin

package main

import (
	"fmt"
	"os"
	"syscall"
)

func lowerPriority() error {
	niceValue := 10 // This is a low priority setting (range is -20 to 19 where -20 is the most favorably scheduled)
	err := syscall.Setpriority(syscall.PRIO_PROCESS, os.Getpid(), niceValue)
	if err != nil {
		return fmt.Errorf("Setpriority for pid: %v returned: %v", os.Getpid(), err)
	}

	return nil
}
