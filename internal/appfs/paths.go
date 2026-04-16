package appfs

import (
	"github.com/prefapp/pad/internal/config"
)

type Paths struct {
	ConfigFile string
}

func Discover() (Paths, error) {
	configFile, err := config.ConfigFile()
	if err != nil {
		return Paths{}, err
	}

	return Paths{
		ConfigFile: configFile,
	}, nil
}
