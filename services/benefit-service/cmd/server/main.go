package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	log, err := logger.NewLogger("info")
	if err != nil {
		logrus.WithError(err).Fatal("initialize startup logger")
	}

	configPath, err := parseConfigPath(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		log.WithError(err).Error("invalid command-line arguments")
		if errors.Is(err, errConfigPathRequired) {
			os.Exit(1)
		}
		os.Exit(2)
	}

	startupLog := log.WithField("config_path", configPath)
	startupLog.Info("loading configuration")

	if err := run(context.Background(), configPath); err != nil {
		startupLog.WithError(err).Fatal("application failed")
	}
}
