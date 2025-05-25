//go:build windows
// +build windows

package priority

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/google/logger"
	"golang.org/x/sys/windows"
)

type PROCESS_POWER_THROTTLING_STATE struct {
	Version     uint32
	ControlMask uint32
	StateMask   uint32
}

// lowerPriority sets the process to the lowest scheduler priority
func LowerPriority() error {
	ph, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_SET_INFORMATION, false, uint32(os.Getpid()))
	if err != nil {
		return fmt.Errorf("windows.OpenProcess for pid: %v returned: %v", uint32(os.Getpid()), err)
	}
	defer func() {
		if err := windows.CloseHandle(ph); err != nil {
			logger.Errorf("failed to close handle after lowering priority: %v", err)
		}
	}()

	err = windows.SetPriorityClass(ph, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		return fmt.Errorf("windows.SetPriorityClass for pid: %v returned: %v", uint32(os.Getpid()), err)
	}

	// Ecoqos / ecomode / power state throttling. 77 is apparently the correct magic number here.
	t := PROCESS_POWER_THROTTLING_STATE{1, 1, 1}
	return windows.NtSetInformationProcess(ph, 77, unsafe.Pointer(&t), uint32(unsafe.Sizeof(t)))
}
