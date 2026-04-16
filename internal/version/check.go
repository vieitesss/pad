package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prefapp/pad/internal/appfs"
)

const (
	githubRepo    = "prefapp/pad"
	checkInterval = 24 * time.Hour
)

type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
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

func fetchLatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	resp, err := http.Get(url)
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
