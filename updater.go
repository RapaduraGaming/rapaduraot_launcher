package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// VersionInfo is the response from GET /api/v1/client/version.
type VersionInfo struct {
	Version         string `json:"version"`
	DownloadURL     string `json:"downloadUrl"`
	Checksum        string `json:"checksum"`        // "sha256:<hex>" or empty to skip
	RSAKeyHash      string `json:"rsaKeyHash"`      // SHA256 of the expected OTSERV_RSA value
	LauncherVersion string `json:"launcherVersion"` // latest launcher version
	LauncherURL     string `json:"launcherUrl"`     // direct URL to new RapaduraOTLauncher.exe
}

func fetchVersionInfo(apiBase string) (*VersionInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(apiBase + "/api/v1/client/version")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// needsUpdate returns true if the installed version differs from info.Version
// OR if the installed RSA key hash doesn't match info.RSAKeyHash.
func needsUpdate(info *VersionInfo, installDir string) bool {
	if info.Version != readLocalVersion(filepath.Join(installDir, versionFile)) {
		return true
	}
	if info.RSAKeyHash != "" {
		localHash := hashInstalledRSAKey(installDir)
		return localHash != info.RSAKeyHash
	}
	return false
}

// hashInstalledRSAKey reads modules/gamelib/const.lua from the installed client,
// extracts the OTSERV_RSA value, and returns its SHA256 hex hash.
func hashInstalledRSAKey(installDir string) string {
	constPath := filepath.Join(installDir, "modules", "gamelib", "const.lua")
	data, err := os.ReadFile(constPath)
	if err != nil {
		return ""
	}

	// Match OTSERV_RSA = '...' spanning multiple concatenated string literals.
	re := regexp.MustCompile(`(?s)OTSERV_RSA\s*=\s*'([^']+)'(?:\s*\.\.\s*'([^']+)')*`)
	m := re.FindString(string(data))
	if m == "" {
		return ""
	}

	// Extract and join all quoted parts.
	parts := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(m, -1)
	var rsaVal strings.Builder
	for _, p := range parts {
		rsaVal.WriteString(p[1])
	}

	h := sha256.Sum256([]byte(rsaVal.String()))
	return hex.EncodeToString(h[:])
}

func readLocalVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "0.0.0"
	}
	return strings.TrimSpace(string(data))
}

func writeLocalVersion(path, version string) {
	os.WriteFile(path, []byte(version), 0644)
}

// downloadAndInstall downloads the update zip and extracts it into installDir.
// progress is called with values 0.0-1.0 during download.
func downloadAndInstall(info *VersionInfo, installDir string, progress func(float64)) error {
	tmpFile := filepath.Join(os.TempDir(), "rapaduraot-update.zip")
	defer os.Remove(tmpFile)

	if err := downloadWithProgress(info.DownloadURL, tmpFile, progress); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	if info.Checksum != "" {
		if err := verifyChecksum(tmpFile, info.Checksum); err != nil {
			return fmt.Errorf("checksum: %w", err)
		}
	}

	return extractZip(tmpFile, installDir)
}

func downloadWithProgress(url, dest string, progress func(float64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if total > 0 && progress != nil {
				progress(float64(downloaded) / float64(total))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func verifyChecksum(path, expected string) error {
	expected = strings.TrimPrefix(expected, "sha256:")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("expected %s, got %s", expected, actual)
	}
	return nil
}

// skipOnExtract lists files that should never be overwritten during an update.
var skipOnExtract = map[string]bool{
	"config.otml": true,
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if skipOnExtract[filepath.Base(f.Name)] {
			continue
		}

		target := filepath.Join(dest, filepath.Clean("/"+f.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue // zip-slip guard
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
