package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vieitesss/pad/internal/appfs"
)

const (
	githubRepo    = "vieitesss/pad"
	checkInterval = 24 * time.Hour
)

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	PublishedAt time.Time      `json:"published_at"`
	HTMLURL     string         `json:"html_url"`
	Assets      []ReleaseAsset `json:"assets"`
}

type VersionState struct {
	LatestVersion string    `json:"latest_version"`
	LastCheck     time.Time `json:"last_check"`
	Skipped       bool      `json:"skipped"`
}

func Current() string {
	return currentVersion
}

func SetCurrent(v string) {
	currentVersion = v
}

var currentVersion = "dev"

func CheckUpdate() (*ReleaseInfo, bool) {
	if currentVersion == "dev" || strings.HasSuffix(currentVersion, "-snapshot") {
		return nil, false
	}

	state, err := loadState()
	if err == nil && time.Since(state.LastCheck) < checkInterval {
		if state.LatestVersion != "" && isNewer(state.LatestVersion, currentVersion) {
			return &ReleaseInfo{TagName: state.LatestVersion}, true
		}
		return nil, false
	}

	release, err := fetchLatestRelease()
	if err != nil {
		return nil, false
	}

	_ = saveState(VersionState{
		LatestVersion: release.TagName,
		LastCheck:     time.Now(),
	})

	if isNewer(release.TagName, currentVersion) {
		return release, true
	}

	return nil, false
}

func LatestRelease() (*ReleaseInfo, error) {
	return fetchRelease(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo))
}

func ReleaseByTag(tag string) (*ReleaseInfo, error) {
	return fetchRelease(fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", githubRepo, url.PathEscape(tag)))
}

func (r ReleaseInfo) AssetForRuntime(goos, goarch string) (ReleaseAsset, error) {
	name, err := assetNameFor(r.TagName, goos, goarch)
	if err != nil {
		return ReleaseAsset{}, err
	}

	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, nil
		}
	}

	return ReleaseAsset{}, fmt.Errorf("release %s does not contain asset %s", r.TagName, name)
}

func fetchLatestRelease() (*ReleaseInfo, error) {
	return LatestRelease()
}

func fetchRelease(apiURL string) (*ReleaseInfo, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pad")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func assetNameFor(tagName, goos, goarch string) (string, error) {
	trimmedTag := strings.TrimSpace(strings.TrimPrefix(tagName, "v"))
	if trimmedTag == "" {
		return "", fmt.Errorf("release tag is empty")
	}

	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	return fmt.Sprintf("pad_%s_%s_%s%s", trimmedTag, goos, goarch, ext), nil
}

func isNewer(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")

	return latest != current && latest != ""
}

func SkipThisVersion(version string) error {
	state, _ := loadState()
	state.LatestVersion = version
	state.LastCheck = time.Now()
	state.Skipped = true
	return saveState(state)
}

func statePath() (string, error) {
	paths, err := appfs.Discover()
	if err != nil {
		return "", err
	}
	return paths.ConfigFile + ".version", nil
}

func loadState() (VersionState, error) {
	path, err := statePath()
	if err != nil {
		return VersionState{}, err
	}

	data, err := appfs.ReadFile(path)
	if err != nil {
		return VersionState{}, err
	}

	var state VersionState
	if err := json.Unmarshal(data, &state); err != nil {
		return VersionState{}, err
	}

	return state, nil
}

func saveState(state VersionState) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return appfs.WriteFile(path, data, 0644)
}
