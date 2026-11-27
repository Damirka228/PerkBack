package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Damirka228/PerkBack/services/user-service/internal/config"
	"github.com/Damirka228/PerkBack/services/user-service/internal/handlers/user"
	"github.com/Damirka228/PerkBack/services/user-service/internal/logger"
	"github.com/Damirka228/PerkBack/services/user-service/internal/server"
	"github.com/Damirka228/PerkBack/services/user-service/internal/storage"
	"github.com/Damirka228/PerkBack/services/user-service/internal/usecase"
	"google.golang.org/grpc"
)

func run(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config %w", err)
	}
	config.ApplyEnv(&cfg)

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	log, err := logger.NewLogger(cfg.Log.Level)
	if err != nil {
		return fmt.Errorf("start logger: %w", err)
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
	userHandler := user.NewHandler(service)
	grpcServer := server.NewGRPCServer(cfg.GRPC.Address, userHandler, log)

	log.WithField("addr", cfg.GRPC.Address).Info("starting gRPC server")

	errCh := make(chan error, 1)
	go func() {
		err := grpcServer.Run()
		if errors.Is(err, grpc.ErrServerStopped) {
			err = nil
		}
		errCh <- err
	}()
	var runErr error
	received := 0

	select {
	case <-signalCtx.Done():
		log.Info("shutdown requested by signal or context")
	case runErr = <-errCh:
		received++
		log.WithError(runErr).Warn("application component stopped")
	}
	stop()
	shutdownErr := server.Shutdown(grpcServer, cfg.ShutdownTimeout, log)
	for received < 1 {
		runErr = errors.Join(runErr, <-errCh)
		received++
	}
	return errors.Join(runErr, shutdownErr)
}
