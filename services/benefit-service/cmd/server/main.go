package main

import (
	"context"
	"os"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	log, err := logger.NewLogger("info")
	if err != nil {
		logrus.WithError(err).Fatal("initialize startup logger")
	}

	configPath := fetchConfigPath()
	if configPath == "" {
		log.Error("config path is required, use -config or CONFIG_PATH")
		os.Exit(1)
	}

	if err := run(context.Background(), configPath); err != nil {
		log.WithError(err).Fatal("benefit-service startup failed")
	}
}
