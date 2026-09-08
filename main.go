package main

import (
	"os"

	"github.com/ZeitOnline/terragrunt-atlantis-gen/internal/cli"
)

// Overridden at build time by GoReleaser.
var appVersion = "dev"

func main() {
	os.Exit(cli.Main(appVersion))
}
