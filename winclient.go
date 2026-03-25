//go:build windows

package main

import (
	"os/exec"
	"time"
	"unsafe"

	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
)

const singleInstanceMutex = "RapaduraOTSingleInstance"

var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procFindWindow = user32.NewProc("FindWindowW")
)

// acquireSingleInstanceMutex tries to create a named mutex so only one
// instance of the launcher/game runs at a time. Returns false if another
// instance is already running (and brings it to the foreground).
func acquireSingleInstanceMutex() (windows.Handle, bool) {
	namePtr, _ := windows.UTF16PtrFromString(singleInstanceMutex)
	h, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil || windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		// Bring existing client window to front if it exists.
		if hwnd := findWindowByTitle(windowTitle); hwnd != 0 {
			w32.ShowWindow(w32.HWND(hwnd), w32.SW_RESTORE)
			w32.SetForegroundWindow(w32.HWND(hwnd))
		}
		return 0, false
	}
	return h, true
}

func findWindowByTitle(title string) uintptr {
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	ret, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	return ret
}

// isClientAlreadyOpen returns true if the game window is already visible.
func isClientAlreadyOpen() bool {
	return findWindowByTitle(windowTitle) != 0
}

// waitForClientWindow launches the client exe and blocks until its window
// appears (or a 30-second timeout). The caller closes the launcher after this
// returns.
func waitForClientWindow(exePath string) {
	if isClientAlreadyOpen() {
		// Client is already running; just bring it forward.
		if hwnd := findWindowByTitle(windowTitle); hwnd != 0 {
			w32.ShowWindow(w32.HWND(hwnd), w32.SW_RESTORE)
			w32.SetForegroundWindow(w32.HWND(hwnd))
		}
		return
	}

	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		return
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if findWindowByTitle(windowTitle) != 0 {
			time.Sleep(600 * time.Millisecond)
			return
		}
	}
}
