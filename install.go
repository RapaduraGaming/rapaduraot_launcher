//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// defaultInstallDir returns the standard installation directory for the game client.
func defaultInstallDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(localAppData, "RapaduraOT")
}

// isClientInstalled returns true if the client exe exists at installDir.
func isClientInstalled(installDir string) bool {
	_, err := os.Stat(filepath.Join(installDir, clientExe))
	return err == nil
}

// selfInstall copies the running launcher exe into installDir if it isn't
// already there, so shortcuts can point to a stable location.
func selfInstall(installDir string) error {
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}

	src, err := os.Executable()
	if err != nil {
		return err
	}

	dst := filepath.Join(installDir, "RapaduraOTLauncher.exe")
	if _, err := os.Stat(dst); err == nil {
		return nil // already there
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

// setupShortcuts creates Desktop and Start Menu shortcuts pointing to the launcher.
func setupShortcuts(installDir string) {
	launcherPath := filepath.Join(installDir, "RapaduraOTLauncher.exe")
	iconPath := filepath.Join(installDir, clientExe) // use client's icon

	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "RapaduraOT.lnk")
	createShortcut(desktop, launcherPath, iconPath)

	startMenu := filepath.Join(
		os.Getenv("APPDATA"),
		"Microsoft", "Windows", "Start Menu", "Programs",
		"RapaduraOT",
	)
	os.MkdirAll(startMenu, 0755)
	createShortcut(
		filepath.Join(startMenu, "RapaduraOT.lnk"),
		launcherPath, iconPath,
	)
}

// createShortcut creates a Windows .lnk file using PowerShell's WScript.Shell COM object.
// This works identically on Windows 10 and 11.
func createShortcut(lnkPath, targetPath, iconPath string) {
	script := fmt.Sprintf(
		`$ws = New-Object -ComObject WScript.Shell; `+
			`$s = $ws.CreateShortcut('%s'); `+
			`$s.TargetPath = '%s'; `+
			`$s.IconLocation = '%s,0'; `+
			`$s.Save()`,
		lnkPath, targetPath, iconPath,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Run()
}
