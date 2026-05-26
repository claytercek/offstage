package main

import (
	"os"

	"github.com/claytercek/offstage/internal/cli"
)

var version = "dev"

var rootCmd = cli.NewRootCmd(version)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
