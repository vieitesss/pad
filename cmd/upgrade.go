package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/prefapp/pad/internal/version"
	"github.com/spf13/cobra"
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

	release, hasUpdate := version.CheckUpdate()
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

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos == "darwin" {
		goos = "Darwin"
	}
	if goarch == "amd64" {
		goarch = "x86_64"
	}

	url := fmt.Sprintf(
		"https://github.com/prefapp/pad/releases/download/%s/pad_%s_%s.tar.gz",
		tagName, goos, goarch,
	)

	tmpDir, err := os.MkdirTemp("", "pad-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "pad.tar.gz")
	if err := downloadFile(url, tarPath); err != nil {
		return fmt.Errorf("download release: %w", err)
	}

	newBinPath := filepath.Join(tmpDir, "pad")
	if err := extractBinary(tarPath, newBinPath); err != nil {
		return fmt.Errorf("extract binary: %w", err)
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

func extractBinary(tarPath, dest string) error {
	file, err := os.Open(tarPath)
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

		if header.Typeflag == tar.TypeReg && strings.Contains(header.Name, "pad") {
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("binary not found in archive")
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
