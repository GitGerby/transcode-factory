//go:build windows
// +build windows

package priority

import (
	"os"

	"testing"

	"golang.org/x/sys/windows"
)

func TestLowerPriority(t *testing.T) {
	ph, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_SET_INFORMATION, false, uint32(os.Getpid()))
	if err != nil {
		t.Errorf("OpenProcess failed: %v", err)
	}

	oldPriority, err := windows.GetPriorityClass(ph)
	if err != nil {
		t.Errorf("GetPriorityClass failed: %v", err)
	}

	t.Cleanup(func() {
		err := windows.SetPriorityClass(ph, oldPriority)
		if err != nil {
			t.Errorf("returning priority after test failed: %v", err)
		}

		err = windows.CloseHandle(ph)
		if err != nil {
			t.Errorf("closing handle after test failed: %v", err)
		}
	})

	err = LowerPriority()
	if err != nil {
		t.Errorf("lowerPriority failed: %v", err)
	}

	newPriority, err := windows.GetPriorityClass(ph)
	if err != nil {
		t.Errorf("GetPriorityClass failed: %v", err)
	}

	if newPriority != windows.IDLE_PRIORITY_CLASS {
		t.Errorf("priority after test failed: got %d, want %d", newPriority, windows.IDLE_PRIORITY_CLASS)
	}
}
