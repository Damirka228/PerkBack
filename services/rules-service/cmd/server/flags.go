package main

import (
	"flag"
	"os"
)

func fetchConfigPath() string {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to the YAML configuration file")
	flag.Parse()

	if configPath == "" {
		return os.Getenv("CONFIG_PATH")
	}

	return configPath
}
