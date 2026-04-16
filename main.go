package main

import (
	"fmt"
	"os"

	"github.com/vieitesss/pad/cmd"
	"github.com/vieitesss/pad/internal/version"
)

var buildVersion = "dev"

func main() {
	version.SetCurrent(buildVersion)

	if err := cmd.NewRootCmd(buildVersion).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
