package appfs

import (
	"testing"
)

func TestDiscoverHonorsXDGOverrides(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/pad-config")

	paths, err := Discover()
	if err != nil {
		t.Fatalf("discover paths: %v", err)
	}

	if paths.ConfigFile != "/tmp/pad-config/pad/pad.toml" {
		t.Fatalf("expected config path /tmp/pad-config/pad/pad.toml, got %q", paths.ConfigFile)
	}
}
