package main

import (
	"fmt"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/logger"
	"github.com/sirupsen/logrus"
)

func main() {

	if err := run("services/benefit-service/configs/local.yaml"); err != nil {
		logrus.WithError(err).Fatal("application startup failed")
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)

	}
	config.ApplyEnv(&cfg)

	if err = cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	log, err := logger.NewLogger(cfg.Log.Level)
	if err != nil {
		return fmt.Errorf("error start logger: %w", err)
	}

	log.WithFields(logrus.Fields{
		"http_addr": cfg.HTTP.Addr,
		"grpc_addr": cfg.GRPC.Addr,
	}).Debug("server addresses loaded")

	log.Info("configuration loaded successfully")

	return nil
}
