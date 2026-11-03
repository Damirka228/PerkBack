package main

import (
	"fmt"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/router"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/logger"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/server"
	"github.com/sirupsen/logrus"
)

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

	httpRouter := router.NewRouter()
	httpServer := server.NewHTTPServer(
		cfg.HTTP.Addr, httpRouter, cfg.Timeout,
	)
	log.WithField("addr", cfg.HTTP.Addr).Info("starting HTTP server")

	return httpServer.Run()
}
