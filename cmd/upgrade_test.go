package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractBinaryFromTarGz(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "pad.tar.gz")
	writeTarGzArchive(t, archivePath, map[string]string{
		"README.md": "docs",
		"pad":       "darwin-binary",
	})

	dest := filepath.Join(t.TempDir(), "pad")
	if err := extractBinary(archivePath, "pad", dest); err != nil {
		t.Fatalf("extract binary: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}

	if string(data) != "darwin-binary" {
		t.Fatalf("unexpected extracted content %q", string(data))
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "pad.zip")
	writeZipArchive(t, archivePath, map[string]string{
		"README.md": "docs",
		"pad.exe":   "windows-binary",
	})

	dest := filepath.Join(t.TempDir(), "pad.exe")
	if err := extractBinary(archivePath, "pad.exe", dest); err != nil {
		t.Fatalf("extract binary: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}

	if string(data) != "windows-binary" {
		t.Fatalf("unexpected extracted content %q", string(data))
	}
}

func writeTarGzArchive(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	defer archive.Close()

	gz := gzip.NewWriter(archive)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, content := range files {
		header := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
}

func writeZipArchive(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer archive.Close()

	zw := zip.NewWriter(archive)
	defer zw.Close()

	for name, content := range files {
		writer, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write zip content: %v", err)
		}
	}
}
