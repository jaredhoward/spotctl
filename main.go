package main

import (
	"log"
	"os"

	"github.com/jaredhoward/spotctl/cmd"
)

var version = "dev"

func main() {
	log.SetFlags(0)
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
