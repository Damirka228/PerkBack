package main

import (
	"context"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	log, err := logger.NewLogger("info")
	if err != nil {
		logrus.WithError(err).Fatal("initialize startup logger")
	}

	configPath := fetchConfigPath()
	if configPath == "" {
		log.Fatal("config path is required, use -config or CONFIG_PATH")
	}

	startupLog := log.WithField("config_path", configPath)
	startupLog.Info("loading configuration")

	if err := run(context.Background(), configPath); err != nil {
		startupLog.WithError(err).Fatal("rules-service startup failed")
	}
}
