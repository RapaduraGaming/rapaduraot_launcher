//go:build windows

package main

import (
	"os"
	"path/filepath"
	"unsafe"

	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
)

const (
	WM_TRAYNOTIFY = w32.WM_USER + 1
	trayUID       = 1

	IDM_PLAY = 100
	IDM_SHOW = 101
	IDM_EXIT = 102

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004
	NIM_ADD     = 0x00000000
	NIM_DELETE  = 0x00000002

	LR_LOADFROMFILE = 0x00000010
	IMAGE_ICON      = 1
)

// NOTIFYICONDATAW is the Win32 struct for Shell_NotifyIcon (Unicode, cbSize-limited).
type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             w32.HWND
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
}

var (
	shell32         = windows.NewLazySystemDLL("shell32.dll")
	procShellNotify = shell32.NewProc("Shell_NotifyIconW")

	user32dll       = windows.NewLazySystemDLL("user32.dll")
	procLoadImage   = user32dll.NewProc("LoadImageW")

	trayHICON   uintptr
	trayHWND    w32.HWND
	trayAdded   bool
	trayIcoTemp string // temp file path, cleaned up on exit
)

func initTray(hwnd w32.HWND) {
	trayHWND = hwnd
	trayHICON = loadHICON()
}

func cleanupTray() {
	removeTrayIcon()
	if trayIcoTemp != "" {
		os.Remove(trayIcoTemp)
	}
}

func loadHICON() uintptr {
	tmp := filepath.Join(os.TempDir(), "rapaduraot-tray.ico")
	if err := os.WriteFile(tmp, iconBytes, 0600); err != nil {
		return 0
	}
	trayIcoTemp = tmp

	pathPtr, _ := windows.UTF16PtrFromString(tmp)
	hicon, _, _ := procLoadImage.Call(
		0,
		uintptr(unsafe.Pointer(pathPtr)),
		IMAGE_ICON,
		0, 0,
		LR_LOADFROMFILE,
	)
	return hicon
}

func addTrayIcon() {
	if trayAdded || trayHWND == 0 {
		return
	}
	nid := buildNID(NIF_MESSAGE | NIF_ICON | NIF_TIP)
	procShellNotify.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	trayAdded = true
}

func removeTrayIcon() {
	if !trayAdded {
		return
	}
	nid := buildNID(0)
	procShellNotify.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	trayAdded = false
}

func buildNID(flags uint32) NOTIFYICONDATAW {
	nid := NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATAW{})),
		HWnd:             trayHWND,
		UID:              trayUID,
		UFlags:           flags,
		UCallbackMessage: WM_TRAYNOTIFY,
		HIcon:            trayHICON,
	}
	tip := windows.StringToUTF16("RapaduraOT")
	copy(nid.SzTip[:], tip)
	return nid
}

func showTrayMenu() {
	hMenu := w32.CreatePopupMenu()
	if hMenu == 0 {
		return
	}
	defer w32.DestroyMenu(hMenu)

	w32.AppendMenu(hMenu, w32.MF_STRING, IDM_PLAY, "Abrir RapaduraOT")
	w32.AppendMenu(hMenu, w32.MF_STRING, IDM_SHOW, "Mostrar Launcher")
	w32.AppendMenu(hMenu, w32.MF_SEPARATOR, 0, "")
	w32.AppendMenu(hMenu, w32.MF_STRING, IDM_EXIT, "Sair")

	const TPM_BOTTOMALIGN uint = 0x0020
	const TPM_RIGHTALIGN uint = 0x0008

	x, y, _ := w32.GetCursorPos()
	w32.SetForegroundWindow(trayHWND)
	w32.TrackPopupMenu(hMenu, TPM_BOTTOMALIGN|TPM_RIGHTALIGN, x, y, trayHWND, nil)
}

func hideLauncherWindow() {
	if trayHWND != 0 {
		w32.ShowWindow(trayHWND, w32.SW_HIDE)
	}
}

func restoreLauncherWindow() {
	if trayHWND != 0 {
		w32.ShowWindow(trayHWND, w32.SW_SHOW)
		w32.SetForegroundWindow(trayHWND)
	}
}
