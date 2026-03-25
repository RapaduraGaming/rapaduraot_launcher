//go:build windows

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// checkAndApplySelfUpdate downloads and applies a launcher update if the API
// reports a newer version. It does not return on success: it launches an
// update script and exits the current process so the new binary takes over.
// Returns false when no update is needed or the update attempt fails
// (allowing the launcher to continue normally).
func checkAndApplySelfUpdate(info *VersionInfo) bool {
	if info.LauncherVersion == "" || info.LauncherURL == "" {
		return false
	}
	if info.LauncherVersion == launcherVersion {
		return false
	}

	currentExe, err := os.Executable()
	if err != nil {
		return false
	}

	newExePath := filepath.Join(os.TempDir(), "RapaduraOTLauncher_update.exe")
	if err := downloadFile(info.LauncherURL, newExePath); err != nil {
		return false
	}

	batPath := filepath.Join(os.TempDir(), "rapaduraot_selfupdate.bat")
	bat := fmt.Sprintf(
		"@echo off\r\n"+
			"timeout /t 2 /nobreak >nul\r\n"+
			"copy /y \"%s\" \"%s\"\r\n"+
			"start \"\" \"%s\"\r\n"+
			"del \"%%~f0\"\r\n",
		newExePath, currentExe, currentExe,
	)
	if err := os.WriteFile(batPath, []byte(bat), 0755); err != nil {
		return false
	}

	exec.Command("cmd", "/c", "start", "", "/b", batPath).Start()
	os.Exit(0)
	return true
}

// downloadFile downloads url to dest, streaming directly to disk.
func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if rerr != nil {
			if rerr.Error() == "EOF" {
				break
			}
			return rerr
		}
	}
	return nil
}
