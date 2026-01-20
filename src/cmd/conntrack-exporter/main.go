package main

import (
	"flag"
	"os"

	"conntrack-exporter/internal/app"
	"conntrack-exporter/internal/config"
)

var (
	// version is meant to be overridden at build time via -ldflags.
	version = "dev"
)

func main() {
	cfg := config.ParseFlags()
	if cfg.ShowHelp {
		flag.Usage()
		os.Exit(0)
	}

	os.Exit(app.Run(cfg, version))
}

