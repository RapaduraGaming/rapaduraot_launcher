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
	status      string
	progress    float64 // -1 = hide bar, 0..1 = show bar with value
	showPlay    bool
	showInstall bool
}

var (
	installDir string
	eventCh    = make(chan uiEvent, 32)
	installCh  = make(chan struct{}, 1) // signals user confirmed installation
)

func main() {
	h, ok := acquireSingleInstanceMutex()
	if !ok {
		return
	}
	defer windows.CloseHandle(h)

	installDir = defaultInstallDir()

	fonts, _ := wui.NewFont(wui.FontDesc{Name: "Segoe UI", Height: -13})

	win := wui.NewWindow()
	win.SetTitle(windowTitle + " Launcher")
	win.SetSize(winW, winH)
	win.SetResizable(false)
	win.SetHasMaxButton(false)
	win.SetFont(fonts)

	if icon := LoadEmbeddedIcon(); icon != nil {
		win.SetIcon(icon)
	} else if icon, err := wui.NewIconFromFile(filepath.Join(installDir, clientExe)); err == nil {
		win.SetIcon(icon)
	}

	const logoH = winH / 3

	statusLbl := wui.NewLabel()
	statusLbl.SetText("Verificando atualizações...")
	statusLbl.SetBounds(20, logoH+20, winW-40, 22)
	statusLbl.SetAlignment(wui.AlignCenter)
	win.Add(statusLbl)

	playBtn := wui.NewButton()
	playBtn.SetText("  JOGAR  ")
	playBtn.SetBounds((winW-120)/2, winH-60, 120, 36)
	playBtn.SetVisible(false)
	win.Add(playBtn)

	playBtn.SetOnClick(func() {
		playBtn.SetVisible(false)
		statusLbl.SetText("Abrindo RapaduraOT...")
		go func() {
			waitForClientWindow(filepath.Join(installDir, clientExe))
			win.Close()
		}()
	})

	installBtn := wui.NewButton()
	installBtn.SetText("  INSTALAR  ")
	installBtn.SetBounds((winW-120)/2, winH-60, 120, 36)
	installBtn.SetVisible(false)
	win.Add(installBtn)

	installBtn.SetOnClick(func() {
		installBtn.SetVisible(false)
		select {
		case installCh <- struct{}{}:
		default:
		}
	})

	win.SetOnMessage(func(window uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
		switch msg {
		case w32.WM_CREATE:
			initUI(w32.HWND(window))
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
				tickAnimation()
				for {
					select {
					case ev := <-eventCh:
						if ev.status != "" {
							statusLbl.SetText(ev.status)
						}
						if ev.progress < 0 {
							setProgress(-1)
						} else if ev.progress > 0 {
							setProgress(ev.progress)
						}
						if ev.showPlay {
							playBtn.SetVisible(true)
						}
						if ev.showInstall {
							installBtn.SetVisible(true)
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
	selfInstall(installDir)

	localVer := readLocalVersion(filepath.Join(installDir, versionFile))

	info, err := fetchVersionInfo(apiBase)
	if err != nil {
		if isClientInstalled(installDir) {
			sendEvent(uiEvent{
				status:   fmt.Sprintf("v%s - Pronto para jogar", localVer),
				progress: -1,
				showPlay: true,
			})
		} else {
			sendEvent(uiEvent{
				status:   "Servidor indisponível. Verifique sua conexão.",
				progress: -1,
			})
		}
		return
	}

	if !isClientInstalled(installDir) {
		sendEvent(uiEvent{
			status:      fmt.Sprintf("RapaduraOT v%s disponível.", info.Version),
			progress:    -1,
			showInstall: true,
		})
		<-installCh // wait for user to click install

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
