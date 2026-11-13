package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/config"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/handlers/rules"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/logger"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/server"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/usecase"
	"google.golang.org/grpc"
)

func run(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.ApplyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	log, err := logger.NewLogger(cfg.Log.Level)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := storage.NewStorage(signalCtx, cfg.DB)
	if err != nil {
		if errors.Is(err, context.Canceled) && signalCtx.Err() != nil {
			log.Info("startup canceled")
			return nil
		}
		return fmt.Errorf("initialize storage: %w", err)
	}
	defer func() {
		log.Info("closing PostgreSQL pool")
		repo.Close()
		log.Info("PostgreSQL pool closed")
	}()

	service := usecase.NewService(repo)
	rulesHandler := rules.NewHandler(service)
	grpcServer := server.NewGRPCServer(cfg.GRPC.Address, rulesHandler, log)

	log.WithField("addr", cfg.GRPC.Address).Info("starting gRPC server")
	errCh := make(chan error, 1)
	go func() { errCh <- grpcServer.Run() }()

	var runErr error
	serverExited := false
	select {
	case <-signalCtx.Done():
		log.WithField("reason", "signal_or_context").Info("shutdown requested")
	case runErr = <-errCh:
		serverExited = true
		log.WithField("reason", "server_exit").Warn("shutdown requested")
	}
	stop()

	shutdownErr := server.Shutdown(grpcServer, cfg.ShutdownTimeout, log)

	if !serverExited {
		runErr = <-errCh
	}
	if errors.Is(runErr, grpc.ErrServerStopped) {
		runErr = nil
	}

	if runErr != nil {
		if shutdownErr != nil {
			log.WithError(shutdownErr).Error("additional shutdown error")
		}
		return runErr
	}
	return shutdownErr
}
