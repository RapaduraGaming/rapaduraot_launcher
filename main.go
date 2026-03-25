package main

import (
	"fmt"
	"path/filepath"

	"github.com/gonutz/w32/v2"
	"github.com/gonutz/wui/v2"
	"golang.org/x/sys/windows"
)

const (
	clientExe   = "RapaduraOT_dx_x64.exe"
	versionFile = "version.txt"
	apiBase     = "https://api.rapadura.org"
	windowTitle = "RapaduraOT"

	timerID = 1
	timerMs = 120

	winW = 460
	winH = 300
)

// uiEvent is sent from background goroutines to the UI update loop.
type uiEvent struct {
	status   string
	progress float64 // -1 = hide, 0..1 = show with value
	showPlay bool
}

var (
	installDir string
	eventCh    = make(chan uiEvent, 32)
)

func main() {
	// Single-instance guard: only one launcher (or client) may be open.
	h, ok := acquireSingleInstanceMutex()
	if !ok {
		return
	}
	defer windows.CloseHandle(h)

	// Resolve install directory.
	installDir = defaultInstallDir()

	fonts, _ := wui.NewFont(wui.FontDesc{Name: "Segoe UI", Height: -13})
	titleFont, _ := wui.NewFont(wui.FontDesc{Name: "Trajan Pro", Height: -20, Bold: true})
	if titleFont == nil {
		titleFont, _ = wui.NewFont(wui.FontDesc{Name: "Palatino Linotype", Height: -20, Bold: true})
	}
	if titleFont == nil {
		titleFont, _ = wui.NewFont(wui.FontDesc{Name: "Georgia", Height: -20, Bold: true})
	}

	win := wui.NewWindow()
	win.SetTitle(windowTitle + " Launcher")
	win.SetSize(winW, winH)
	win.SetResizable(false)
	win.SetHasMaxButton(false)
	win.SetFont(fonts)

	// Try to set the window icon from the installed client exe, falling back to
	// a relative path during development.
	if icon, err := wui.NewIconFromFile(filepath.Join(installDir, clientExe)); err == nil {
		win.SetIcon(icon)
	}

	// Leave room at top for the logo (drawn in WM_ERASEBKGND).
	// Layout: logo occupies ~top third, controls sit in the lower two thirds.
	const logoH = winH / 3

	statusLbl := wui.NewLabel()
	statusLbl.SetText("Verificando atualizações...")
	statusLbl.SetBounds(20, logoH+20, winW-40, 22)
	statusLbl.SetAlignment(wui.AlignCenter)
	win.Add(statusLbl)

	progressLbl := wui.NewLabel()
	progressLbl.SetBounds(20, logoH+46, winW-40, 18)
	progressLbl.SetAlignment(wui.AlignCenter)
	progressLbl.SetVisible(false)
	win.Add(progressLbl)

	playBtn := wui.NewButton()
	playBtn.SetText("  JOGAR  ")
	playBtn.SetBounds((winW-120)/2, winH-60, 120, 36)
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

	win.SetOnMessage(func(window uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
		switch msg {
		case w32.WM_CREATE:
			initUI(installDir)
			w32.SetTimer(w32.HWND(window), timerID, timerMs, 0)
			go runLauncher()
			return false, 0

		case w32.WM_DESTROY:
			destroyUI()
			return false, 0

		case w32.WM_ERASEBKGND:
			rect := w32.GetClientRect(w32.HWND(window))
			drawBackground(w32.HDC(wParam),
				int(rect.Right-rect.Left),
				int(rect.Bottom-rect.Top))
			return true, 1

		case w32.WM_CTLCOLORSTATIC:
			return handleCtlColorStatic(wParam)

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

func runLauncher() {
	// Ensure the launcher exe is present in the install directory so shortcuts
	// point to a stable path.
	selfInstall(installDir)

	localVer := readLocalVersion(filepath.Join(installDir, versionFile))

	info, err := fetchVersionInfo(apiBase)
	if err != nil {
		// API unreachable: let the user play with whatever is installed.
		if isClientInstalled(installDir) {
			sendEvent(uiEvent{
				status:   fmt.Sprintf("v%s - Pronto para jogar", localVer),
				progress: -1,
				showPlay: true,
			})
		} else {
			sendEvent(uiEvent{
				status:   "Sem conexão. Instale o jogo primeiro.",
				progress: -1,
			})
		}
		return
	}

	// First-run installation: client not present yet.
	if !isClientInstalled(installDir) {
		sendEvent(uiEvent{
			status:   fmt.Sprintf("Instalando RapaduraOT v%s...", info.Version),
			progress: 0.01,
		})
		if err := downloadAndInstall(info, installDir, func(pct float64) {
			sendEvent(uiEvent{progress: pct})
		}); err != nil {
			sendEvent(uiEvent{
				status:   "Erro ao instalar. Verifique sua conexão.",
				progress: -1,
			})
			return
		}
		writeLocalVersion(filepath.Join(installDir, versionFile), info.Version)
		setupShortcuts(installDir)
		sendEvent(uiEvent{
			status:   fmt.Sprintf("v%s - Pronto para jogar", info.Version),
			progress: -1,
			showPlay: true,
		})
		return
	}

	// Update check: version mismatch OR RSA key hash mismatch.
	if !needsUpdate(info, installDir) {
		sendEvent(uiEvent{
			status:   fmt.Sprintf("v%s - Pronto para jogar", info.Version),
			progress: -1,
			showPlay: true,
		})
		return
	}

	sendEvent(uiEvent{
		status:   fmt.Sprintf("Baixando v%s...", info.Version),
		progress: 0.01,
	})

	if err := downloadAndInstall(info, installDir, func(pct float64) {
		sendEvent(uiEvent{progress: pct})
	}); err != nil {
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
