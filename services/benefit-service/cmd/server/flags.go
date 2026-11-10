package main

import (
	"errors"
	"flag"
	"io"
	"os"
)

var errConfigPathRequired = errors.New("config path is required, use -config")

func parseConfigPath(args []string) (string, error) {
	flags := flag.NewFlagSet("benefit-service", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "", "path to the YAML configuration file")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(os.Stdout)
			flags.Usage()
		}
		return "", err
	}

	if *configPath == "" {
		return "", errConfigPathRequired
	}

	return *configPath, nil
}
