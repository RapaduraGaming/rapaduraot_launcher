package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/gonutz/w32/v2"
	"github.com/gonutz/wui/v2"
)

const (
	clientExe   = "RapaduraOT_dx_x64.exe"
	versionFile = "version.txt"
	apiBase     = "https://api.rapadura.org"
	windowTitle = "RapaduraOT"

	timerID = 1
	timerMs = 120
)

// uiEvent is sent from background goroutines to the UI update loop.
type uiEvent struct {
	status   string
	progress float64 // -1 = hide, 0..1 = show with value
	showPlay bool
}

var (
	installDir  string
	eventCh     = make(chan uiEvent, 32)
	goroutineOK int32 // CAS flag to start goroutine only once
)

func main() {
	exe, _ := os.Executable()
	installDir = filepath.Dir(exe)

	titleFont, _ := wui.NewFont(wui.FontDesc{Name: "Segoe UI", Height: -22, Bold: true})
	bodyFont, _ := wui.NewFont(wui.FontDesc{Name: "Segoe UI", Height: -14})

	win := wui.NewWindow()
	win.SetTitle(windowTitle + " Launcher")
	win.SetSize(436, 256) // outer size; client area ~420x220 on most DPIs
	win.SetResizable(false)
	win.SetHasMaxButton(false)
	win.SetFont(bodyFont)

	titleLbl := wui.NewLabel()
	titleLbl.SetFont(titleFont)
	titleLbl.SetText("RapaduraOT")
	titleLbl.SetBounds(0, 22, 420, 36)
	titleLbl.SetAlignment(wui.AlignCenter)
	win.Add(titleLbl)

	statusLbl := wui.NewLabel()
	statusLbl.SetText("Verificando atualizações...")
	statusLbl.SetBounds(20, 78, 380, 22)
	statusLbl.SetAlignment(wui.AlignCenter)
	win.Add(statusLbl)

	progressLbl := wui.NewLabel()
	progressLbl.SetText("")
	progressLbl.SetBounds(20, 104, 380, 18)
	progressLbl.SetAlignment(wui.AlignCenter)
	progressLbl.SetVisible(false)
	win.Add(progressLbl)

	playBtn := wui.NewButton()
	playBtn.SetText("  JOGAR  ")
	playBtn.SetBounds(155, 148, 110, 34)
	playBtn.SetVisible(false)
	win.Add(playBtn)

	playBtn.SetOnClick(func() {
		playBtn.SetVisible(false)
		statusLbl.SetText("Abrindo RapaduraOT...")
		go func() {
			clientPath := filepath.Join(installDir, clientExe)
			waitForClientWindow(clientPath)
			win.Close()
		}()
	})

	// Use WM_TIMER to poll eventCh on the main thread.
	win.SetOnMessage(func(window uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
		switch msg {
		case w32.WM_CREATE:
			w32.SetTimer(w32.HWND(window), timerID, timerMs, 0)
			// Start background work after the window is created.
			if atomic.CompareAndSwapInt32(&goroutineOK, 0, 1) {
				go checkAndUpdate()
			}
			return false, 0

		case w32.WM_TIMER:
			if wParam == timerID {
				for {
					select {
					case ev := <-eventCh:
						if ev.status != "" {
							statusLbl.SetText(ev.status)
						}
						switch {
						case ev.progress < 0:
							progressLbl.SetVisible(false)
						case ev.progress > 0:
							progressLbl.SetText(fmt.Sprintf("[%.0f%%]", ev.progress*100))
							progressLbl.SetVisible(true)
						}
						if ev.showPlay {
							playBtn.SetVisible(true)
						}
					default:
						return true, 0
					}
				}
			}
		}
		return false, 0
	})

	win.Show()
}

func sendEvent(ev uiEvent) {
	select {
	case eventCh <- ev:
	default:
	}
}

func checkAndUpdate() {
	localVer := readLocalVersion(filepath.Join(installDir, versionFile))

	info, err := fetchVersionInfo(apiBase)
	if err != nil {
		// API unreachable - proceed with installed version
		sendEvent(uiEvent{
			status:   fmt.Sprintf("v%s - Pronto para jogar", localVer),
			progress: -1,
			showPlay: true,
		})
		return
	}

	if info.Version == localVer {
		sendEvent(uiEvent{
			status:   fmt.Sprintf("v%s - Pronto para jogar", localVer),
			progress: -1,
			showPlay: true,
		})
		return
	}

	// Update available
	sendEvent(uiEvent{
		status:   fmt.Sprintf("Baixando v%s...", info.Version),
		progress: 0.01,
	})

	err = downloadAndInstall(info, installDir, func(pct float64) {
		sendEvent(uiEvent{progress: pct})
	})
	if err != nil {
		sendEvent(uiEvent{
			status:   "Erro ao atualizar. Verifique sua conexão.",
			progress: -1,
			showPlay: true,
		})
		return
	}

	writeLocalVersion(filepath.Join(installDir, versionFile), info.Version)
	sendEvent(uiEvent{
		status:   fmt.Sprintf("v%s - Pronto para jogar", info.Version),
		progress: -1,
		showPlay: true,
	})
}
