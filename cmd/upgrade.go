package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/vieitesss/pad/internal/version"
)

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade pad to the latest version",
		RunE:  runUpgrade,
	}
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	current := version.Current()

	fmt.Println("Checking for updates...")

	// Force a fresh check, bypassing the cache
	release, hasUpdate := version.CheckUpdate(true)
	if !hasUpdate {
		green := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
		fmt.Println(green.Render("You're already on the latest version!"))
		return nil
	}

	fmt.Printf("New version available: %s (current: %s)\n", release.TagName, current)
	fmt.Println("Downloading...")

	if err := downloadAndInstall(release.TagName); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	if runtime.GOOS == "windows" {
		yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
		fmt.Println(yellow.Render("Upgrade scheduled. Restart pad after this command exits to finish replacing the binary."))
		return nil
	}

	fmt.Println(green.Render("✓ Upgrade successful! Restart pad to use the new version."))

	return nil
}

func downloadAndInstall(tagName string) error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	realPath, err := filepath.EvalSymlinks(binPath)
	if err != nil {
		realPath = binPath
	}

	release, err := version.ReleaseByTag(tagName)
	if err != nil {
		return fmt.Errorf("load release metadata: %w", err)
	}

	asset, err := release.AssetForRuntime(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "pad-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, archivePath); err != nil {
		return fmt.Errorf("download release: %w", err)
	}

	archiveBinaryName := archivedBinaryName(runtime.GOOS)
	newBinPath := filepath.Join(tmpDir, archiveBinaryName)
	if err := extractBinary(archivePath, archiveBinaryName, newBinPath); err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	if runtime.GOOS == "windows" {
		return stageWindowsUpgrade(newBinPath, realPath)
	}

	backupPath := realPath + ".backup"
	if err := os.Rename(realPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := copyFile(newBinPath, realPath); err != nil {
		os.Rename(backupPath, realPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	os.Remove(backupPath)

	if err := os.Chmod(realPath, 0755); err != nil {
		return fmt.Errorf("set executable permissions: %w", err)
	}

	return nil
}

func archivedBinaryName(goos string) string {
	if goos == "windows" {
		return "pad.exe"
	}

	return "pad"
}

func stageWindowsUpgrade(src, realPath string) error {
	stagedPath := realPath + ".new"
	if err := copyFile(src, stagedPath); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		windowsUpgradeScript(realPath, stagedPath, realPath+".backup"),
	)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(stagedPath)
		return fmt.Errorf("start windows installer: %w", err)
	}

	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}

	return nil
}

func windowsUpgradeScript(realPath, stagedPath, backupPath string) string {
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$target = %s
$staged = %s
$backup = %s
for ($i = 0; $i -lt 20; $i++) {
  try {
    if (Test-Path -LiteralPath $backup) { Remove-Item -LiteralPath $backup -Force }
    Move-Item -LiteralPath $target -Destination $backup -Force
    Move-Item -LiteralPath $staged -Destination $target -Force
    Remove-Item -LiteralPath $backup -Force
    exit 0
  } catch {
    if ((Test-Path -LiteralPath $backup) -and -not (Test-Path -LiteralPath $target)) {
      Move-Item -LiteralPath $backup -Destination $target -Force
    }
    Start-Sleep -Milliseconds 500
  }
}
throw 'timed out replacing pad.exe'
`, powershellString(realPath), powershellString(stagedPath), powershellString(backupPath))
}

func powershellString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(archivePath, binaryName, dest string) error {
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return extractTarGzBinary(archivePath, binaryName, dest)
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZipBinary(archivePath, binaryName, dest)
	default:
		return fmt.Errorf("unsupported archive format for %s", archivePath)
	}
}

func extractTarGzBinary(archivePath, binaryName, dest string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg && path.Base(header.Name) == binaryName {
			return writeReaderToFile(dest, tr, 0o755)
		}
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractZipBinary(archivePath, binaryName, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || path.Base(file.Name) != binaryName {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}

		perm := file.Mode()
		if perm == 0 {
			perm = 0o755
		}

		writeErr := writeReaderToFile(dest, rc, perm)
		closeErr := rc.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}

		return nil
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func writeReaderToFile(dest string, src io.Reader, perm os.FileMode) error {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}

	return closeErr
}

func copyFile(src, dest string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
