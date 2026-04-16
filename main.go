package main

import (
	"fmt"
	"os"

	"github.com/prefapp/pad/cmd"
)

var version = "dev"

func main() {
	if err := cmd.NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
