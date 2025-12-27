package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	GitHubOwner = "glennswest"
	GitHubRepo  = "aicli"
	APIBaseURL  = "https://api.github.com"
)

// Release represents a GitHub release
type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

// Asset represents a release asset (binary download)
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseNotes   string
	DownloadURL    string
	AssetName      string
	AssetSize      int64
}

// CheckForUpdate checks if a newer version is available
func CheckForUpdate(currentVersion string) (*UpdateInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", APIBaseURL, GitHubOwner, GitHubRepo)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases found")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion = strings.TrimPrefix(currentVersion, "v")

	// Find the appropriate asset for this platform
	assetName := getAssetName()
	var downloadURL string
	var assetSize int64

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			assetSize = asset.Size
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseNotes:   release.Body,
		DownloadURL:    downloadURL,
		AssetName:      assetName,
		AssetSize:      assetSize,
	}, nil
}

// IsNewerVersion returns true if latest is newer than current
func IsNewerVersion(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	for i := 0; i < 3; i++ {
		var c, l int
		if i < len(currentParts) {
			fmt.Sscanf(currentParts[i], "%d", &c)
		}
		if i < len(latestParts) {
			fmt.Sscanf(latestParts[i], "%d", &l)
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

// getAssetName returns the expected asset filename for the current platform
func getAssetName() string {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "aicli-darwin-arm64.zip"
		}
		return "aicli-darwin-amd64.zip"
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "aicli-linux-arm64.tar.gz"
		}
		return "aicli-linux-amd64.tar.gz"
	case "windows":
		return "aicli-windows-amd64.zip"
	default:
		return ""
	}
}

// DownloadAndInstall downloads the update and installs it
func DownloadAndInstall(info *UpdateInfo, onProgress func(downloaded, total int64)) error {
	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Download the asset to a temp file
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(info.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "aicli-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Download with progress tracking
	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				tmpFile.Close()
				return fmt.Errorf("failed to write temp file: %w", writeErr)
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, info.AssetSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			tmpFile.Close()
			return fmt.Errorf("download error: %w", err)
		}
	}
	tmpFile.Close()

	// Extract the binary from the archive
	var newBinaryPath string
	if strings.HasSuffix(info.AssetName, ".zip") {
		newBinaryPath, err = extractZip(tmpPath)
	} else if strings.HasSuffix(info.AssetName, ".tar.gz") {
		newBinaryPath, err = extractTarGz(tmpPath)
	} else {
		return fmt.Errorf("unknown archive format: %s", info.AssetName)
	}
	if err != nil {
		return fmt.Errorf("failed to extract update: %w", err)
	}
	defer os.Remove(newBinaryPath)

	// Replace the current executable
	// First, rename the old one as backup
	backupPath := execPath + ".old"
	os.Remove(backupPath) // Remove any existing backup

	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Copy the new binary to the exec path
	if err := copyFile(newBinaryPath, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Make it executable
	if err := os.Chmod(execPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	return nil
}

// extractZip extracts the aicli binary from a zip file
func extractZip(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.Contains(f.Name, "aicli") && !strings.HasSuffix(f.Name, "/") {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			tmpFile, err := os.CreateTemp("", "aicli-binary-*")
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(tmpFile, rc); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return "", err
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}
	}
	return "", fmt.Errorf("aicli binary not found in archive")
}

// extractTarGz extracts the aicli binary from a tar.gz file
func extractTarGz(tgzPath string) (string, error) {
	f, err := os.Open(tgzPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if strings.Contains(header.Name, "aicli") && header.Typeflag == tar.TypeReg {
			tmpFile, err := os.CreateTemp("", "aicli-binary-*")
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(tmpFile, tr); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return "", err
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}
	}
	return "", fmt.Errorf("aicli binary not found in archive")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}
