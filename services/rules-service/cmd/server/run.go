package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	kafkaclient "github.com/Damirka228/PerkBack/services/rules-service/internal/client/kafka"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/config"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/handlers/rules"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/logger"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/server"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/usecase"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/worker"
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

	producer, err := kafkaclient.NewProducer(
		[]string{"localhost:9092"},
		"promotions.event",
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("create producer kafka:%w", err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.WithError(err).Error("close kafka producer")
		}
	}()

	outboxWorker := worker.NewOutboxWorker(repo, producer, 10)

	service := usecase.NewService(repo)
	rulesHandler := rules.NewHandler(service)
	grpcServer := server.NewGRPCServer(cfg.GRPC.Address, rulesHandler, log)

	log.WithField("addr", cfg.GRPC.Address).Info("starting gRPC server")

	errCh := make(chan error, 2)
	go func() {
		err := grpcServer.Run()
		if errors.Is(err, grpc.ErrServerStopped) {
			err = nil
		}
		errCh <- err
	}()
	go func() {
		err := outboxWorker.Run(signalCtx, 500*time.Millisecond, 3*time.Second, log)
		if err != nil {
			err = fmt.Errorf("outbox worker: %w", err)
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
	shutdownErr := server.Shutdown(
		grpcServer,
		cfg.ShutdownTimeout,
		log,
	)
	for received < 2 {
		runErr = errors.Join(runErr, <-errCh)
		received++
	}

	return errors.Join(runErr, shutdownErr)
}
