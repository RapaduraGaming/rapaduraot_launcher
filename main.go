package main

import (
	"fmt"
	"path/filepath"

	"github.com/gonutz/w32/v2"
	"github.com/gonutz/wui/v2"
	"golang.org/x/sys/windows"
)

const (
	clientExe      = "RapaduraOT_dx_x64.exe"
	versionFile    = "version.txt"
	apiBase        = "https://api.rapadura.org"
	windowTitle    = "Rapadura OT"
	launcherVersion = "0.1.0"

	timerID = 1
	timerMs = 120

	winW = 460
	winH = 320
)

// uiEvent is sent from background goroutines to the UI update loop.
type uiEvent struct {
	status      string
	progress    float64 // -1 = hide bar, 0..1 = show bar with value
	showPlay    bool
	showInstall bool
	showRetry   bool
}

var (
	installDir     string
	currentVersion string
	eventCh        = make(chan uiEvent, 32)
	installCh      = make(chan struct{}, 1)
	retryCh        = make(chan struct{}, 1)
	cachedInfo     *VersionInfo // fetched once during self-update check, reused in runLauncher
)

func main() {
	h, ok := acquireSingleInstanceMutex()
	if !ok {
		return
	}
	defer windows.CloseHandle(h)

	installDir = defaultInstallDir()

	// Self-update: check launcher version before showing any UI.
	if info, err := fetchVersionInfo(apiBase); err == nil {
		if checkAndApplySelfUpdate(info) {
			return // process exits inside checkAndApplySelfUpdate
		}
		cachedInfo = info
	}

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
	statusLbl.SetBounds(20, logoH+35, winW-40, 22)
	statusLbl.SetAlignment(wui.AlignCenter)
	win.Add(statusLbl)

	footerFont, _ := wui.NewFont(wui.FontDesc{Name: "Segoe UI", Height: -11})
	footerLbl := wui.NewLabel()
	footerLbl.SetText("Estamos em construção! Seu feedback é muito importante.")
	footerLbl.SetFont(footerFont)
	footerLbl.SetBounds(10, winH-80, winW-20, 14)
	footerLbl.SetAlignment(wui.AlignCenter)
	win.Add(footerLbl)

	footerLinkLbl := wui.NewLabel()
	footerLinkLbl.SetText("Acesse ot.rapadura.org")
	footerLinkLbl.SetFont(footerFont)
	footerLinkLbl.SetBounds(10, winH-65, winW-20, 14)
	footerLinkLbl.SetAlignment(wui.AlignCenter)
	win.Add(footerLinkLbl)

	installBtn := wui.NewButton()
	installBtn.SetText("  INSTALAR  ")
	installBtn.SetBounds((winW-140)/2, winH-135, 140, 36)
	installBtn.SetVisible(false)
	win.Add(installBtn)

	installBtn.SetOnClick(func() {
		installBtn.SetVisible(false)
		select {
		case installCh <- struct{}{}:
		default:
		}
	})

	playBtn := wui.NewButton()
	playBtn.SetText("  JOGAR  ")
	playBtn.SetBounds((winW-120)/2, winH-135, 120, 36)
	playBtn.SetVisible(false)
	win.Add(playBtn)

	playBtn.SetOnClick(func() {
		playBtn.SetVisible(false)
		statusLbl.SetText("Abrindo RapaduraOT...")
		go func() {
			hideLauncherWindow()
			addTrayIcon()
			waitForClientWindow(filepath.Join(installDir, clientExe))
			sendEvent(uiEvent{
				status:   fmt.Sprintf("v%s - Pronto para jogar", currentVersion),
				progress: -1,
				showPlay: true,
			})
		}()
	})

	retryBtn := wui.NewButton()
	retryBtn.SetText("  TENTAR NOVAMENTE  ")
	retryBtn.SetBounds((winW-190)/2, winH-135, 190, 36)
	retryBtn.SetVisible(false)
	win.Add(retryBtn)

	retryBtn.SetOnClick(func() {
		retryBtn.SetVisible(false)
		select {
		case retryCh <- struct{}{}:
		default:
		}
	})

	win.SetOnMessage(func(window uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
		switch msg {
		case w32.WM_CREATE:
			initUI(w32.HWND(window))
			makeButtonOwnerdraw(w32.HWND(installBtn.Handle()))
			makeButtonOwnerdraw(w32.HWND(playBtn.Handle()))
			makeButtonOwnerdraw(w32.HWND(retryBtn.Handle()))
			initTray(w32.HWND(window))
			w32.SetTimer(w32.HWND(window), timerID, timerMs, 0)
			go runLauncher()
			return false, 0

		case w32.WM_DESTROY:
			cleanupTray()
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

		case 0x002B: // WM_DRAWITEM
			return handleDrawItem(lParam, w32.HWND(installBtn.Handle()), w32.HWND(playBtn.Handle()), w32.HWND(retryBtn.Handle()))

		case WM_TRAYNOTIFY:
			switch lParam {
			case w32.WM_RBUTTONUP:
				showTrayMenu()
			case w32.WM_LBUTTONDBLCLK:
				restoreLauncherWindow()
			}
			return true, 0

		case w32.WM_COMMAND:
			switch wParam & 0xFFFF {
			case IDM_PLAY:
				go launchFromTray()
				return true, 0
			case IDM_SHOW:
				restoreLauncherWindow()
				return true, 0
			case IDM_EXIT:
				removeTrayIcon()
				w32.PostMessage(w32.HWND(window), w32.WM_CLOSE, 0, 0)
				return true, 0
			}

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
						if ev.showInstall {
							installBtn.SetVisible(true)
						}
						if ev.showPlay {
							playBtn.SetVisible(true)
						}
						if ev.showRetry {
							retryBtn.SetVisible(true)
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

func launchFromTray() {
	hideLauncherWindow()
	waitForClientWindow(filepath.Join(installDir, clientExe))
	sendEvent(uiEvent{
		status:   fmt.Sprintf("v%s - Pronto para jogar", currentVersion),
		progress: -1,
		showPlay: true,
	})
}

func runLauncher() {
	selfInstall(installDir)

	localVer := readLocalVersion(filepath.Join(installDir, versionFile))

	info := cachedInfo
	if info == nil {
		var err error
		info, err = fetchVersionInfo(apiBase)
		if err != nil {
			if isClientInstalled(installDir) {
				currentVersion = localVer
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
	}

	if !isClientInstalled(installDir) {
		sendEvent(uiEvent{
			status:      fmt.Sprintf("RapaduraOT v%s disponível. Deseja instalar?", info.Version),
			progress:    -1,
			showInstall: true,
		})
		<-installCh
		sendEvent(uiEvent{
			status:   fmt.Sprintf("Baixando RapaduraOT v%s...", info.Version),
			progress: 0.01,
		})
		for {
			sendEvent(uiEvent{
				status:   fmt.Sprintf("Baixando RapaduraOT v%s...", info.Version),
				progress: 0.01,
			})
			err := downloadAndInstall(info, installDir, func(pct float64) {
				sendEvent(uiEvent{
					status:   fmt.Sprintf("Baixando RapaduraOT v%s... %.0f%%", info.Version, pct*100),
					progress: pct,
				})
			})
			if err == nil {
				break
			}
			sendEvent(uiEvent{
				status:    "Erro ao instalar. Verifique sua conexão.",
				progress:  -1,
				showRetry: true,
			})
			<-retryCh
		}
		writeLocalVersion(filepath.Join(installDir, versionFile), info.Version)
		setupShortcuts(installDir)
		currentVersion = info.Version
		sendEvent(uiEvent{
			status:   fmt.Sprintf("v%s instalado! Quer jogar agora?", info.Version),
			progress: -1,
			showPlay: true,
		})
		return
	}

	if !needsUpdate(info, installDir) {
		currentVersion = info.Version
		sendEvent(uiEvent{
			status:   fmt.Sprintf("v%s - Pronto para jogar", info.Version),
			progress: -1,
			showPlay: true,
		})
		return
	}

	for {
		sendEvent(uiEvent{
			status:   fmt.Sprintf("Atualizando para v%s...", info.Version),
			progress: 0.01,
		})
		err := downloadAndInstall(info, installDir, func(pct float64) {
			sendEvent(uiEvent{
				status:   fmt.Sprintf("Atualizando para v%s... %.0f%%", info.Version, pct*100),
				progress: pct,
			})
		})
		if err == nil {
			break
		}
		sendEvent(uiEvent{
			status:    "Erro ao atualizar. Verifique sua conexão.",
			progress:  -1,
			showRetry: true,
		})
		<-retryCh
	}

	writeLocalVersion(filepath.Join(installDir, versionFile), info.Version)
	currentVersion = info.Version
	sendEvent(uiEvent{
		status:   fmt.Sprintf("v%s - Pronto para jogar", info.Version),
		progress: -1,
		showPlay: true,
	})
}
