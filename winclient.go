//go:build windows

package main

import (
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procFindWindow = user32.NewProc("FindWindowW")
)

func findWindowByTitle(title string) bool {
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	ret, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	return ret != 0
}

// waitForClientWindow launches the client and blocks until its window appears
// (or a 30-second timeout). The caller closes the launcher after this returns.
func waitForClientWindow(exePath string) {
	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		return
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if findWindowByTitle(windowTitle) {
			// Give the window a moment to fully render before closing launcher.
			time.Sleep(600 * time.Millisecond)
			return
		}
	}
}
