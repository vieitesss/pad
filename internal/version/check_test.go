package version

import "testing"

func TestAssetNameForUnixArchives(t *testing.T) {
	name, err := assetNameFor("v0.2.1", "darwin", "arm64")
	if err != nil {
		t.Fatalf("asset name: %v", err)
	}

	if name != "pad_0.2.1_darwin_arm64.tar.gz" {
		t.Fatalf("expected darwin asset name, got %q", name)
	}
}

func TestAssetNameForWindowsArchives(t *testing.T) {
	name, err := assetNameFor("v0.2.1", "windows", "amd64")
	if err != nil {
		t.Fatalf("asset name: %v", err)
	}

	if name != "pad_0.2.1_windows_amd64.zip" {
		t.Fatalf("expected windows asset name, got %q", name)
	}
}

func TestReleaseInfoAssetForRuntime(t *testing.T) {
	release := ReleaseInfo{
		TagName: "v0.2.1",
		Assets: []ReleaseAsset{
			{Name: "pad_0.2.1_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux"},
			{Name: "pad_0.2.1_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows"},
		},
	}

	asset, err := release.AssetForRuntime("windows", "amd64")
	if err != nil {
		t.Fatalf("asset for runtime: %v", err)
	}

	if asset.BrowserDownloadURL != "https://example.com/windows" {
		t.Fatalf("expected windows asset, got %q", asset.BrowserDownloadURL)
	}
}
