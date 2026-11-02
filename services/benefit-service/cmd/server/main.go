package main

import (
	"fmt"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := run("services/benefit-service/configs/local.yaml"); err != nil {
		logrus.WithError(err).Fatal("application startup failed")
	}
	logrus.Info("configuration loaded successfully!")
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
	fmt.Printf("HTTP=%s GRPC=%s\n", cfg.HTTP.Addr, cfg.GRPC.Addr)
	return nil
}
