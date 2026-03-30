; RapaduraOT Launcher Installer
; Installs only the launcher. The game client is downloaded on first run.
; -----------------------------------------------

Unicode true
!include "MUI2.nsh"

; -----------------------------------------------
; Settings
; -----------------------------------------------
!define PRODUCT_NAME     "RapaduraOT"
!define LAUNCHER_EXE     "RapaduraOTLauncher.exe"
!define PRODUCT_VERSION  "0.1.0"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

Name "${PRODUCT_NAME}"
OutFile "..\RapaduraOT-Setup.exe"
InstallDir "$LOCALAPPDATA\${PRODUCT_NAME}"
InstallDirRegKey HKCU "Software\${PRODUCT_NAME}" ""
RequestExecutionLevel user
SetCompressor /SOLID lzma

; -----------------------------------------------
; Interface
; -----------------------------------------------
!define MUI_ABORTWARNING
!define MUI_ICON   "..\winres\icon.ico"
!define MUI_UNICON "..\winres\icon.ico"
!define MUI_WELCOMEFINISHPAGE_BITMAP "welcome_sidebar.bmp"

; -----------------------------------------------
; Pages
; -----------------------------------------------
!define MUI_WELCOMEPAGE_TITLE       "Bem-vindo ao RapaduraOT"
!define MUI_WELCOMEPAGE_TEXT        "Este assistente irá instalar o RapaduraOT no seu computador.$\r$\n$\r$\nO cliente do jogo será baixado automaticamente na primeira execução.$\r$\n$\r$\nClique em Instalar para continuar."

!define MUI_FINISHPAGE_RUN          "$INSTDIR\${LAUNCHER_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT     "Iniciar o RapaduraOT"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; -----------------------------------------------
; Language
; -----------------------------------------------
!insertmacro MUI_LANGUAGE "PortugueseBR"

; -----------------------------------------------
; Installer Section
; -----------------------------------------------
Section "Launcher"
  SetOutPath "$INSTDIR"
  File "..\${LAUNCHER_EXE}"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Desktop shortcut named "RapaduraOT" (not "Launcher")
  CreateShortCut "$DESKTOP\${PRODUCT_NAME}.lnk" \
    "$INSTDIR\${LAUNCHER_EXE}" "" "$INSTDIR\${LAUNCHER_EXE}" 0

  ; Start Menu shortcuts
  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" \
    "$INSTDIR\${LAUNCHER_EXE}" "" "$INSTDIR\${LAUNCHER_EXE}" 0
  CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}\Desinstalar.lnk" \
    "$INSTDIR\uninstall.exe"

  ; Add/Remove Programs registry entry
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "DisplayName"    "${PRODUCT_NAME}"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "DisplayIcon"    "$INSTDIR\${LAUNCHER_EXE}"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "Publisher"      "${PRODUCT_NAME}"
  WriteRegStr HKCU "Software\${PRODUCT_NAME}" "InstallDir"  "$INSTDIR"
SectionEnd

; -----------------------------------------------
; Uninstaller Section
; -----------------------------------------------
Section "Uninstall"
  ; Remove launcher and uninstaller
  Delete "$INSTDIR\${LAUNCHER_EXE}"
  Delete "$INSTDIR\uninstall.exe"

  ; Remove shortcuts
  Delete "$DESKTOP\${PRODUCT_NAME}.lnk"
  RMDir /r "$SMPROGRAMS\${PRODUCT_NAME}"

  ; Remove registry entries
  DeleteRegKey HKCU "${PRODUCT_UNINST_KEY}"
  DeleteRegKey HKCU "Software\${PRODUCT_NAME}"

  ; Remove install dir and all client files
  RMDir /r "$INSTDIR"
SectionEnd