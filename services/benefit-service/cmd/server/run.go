package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/benefit"
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

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpRouter := router.NewRouter()
	httpServer := server.NewHTTPServer(
		cfg.HTTP.Addr, httpRouter, cfg.Timeout,
	)
	log.WithField("http addr:", cfg.HTTP.Addr).Info("starting HTTP server")

	gRPCBenefitHandler := benefit.NewHandler()
	gRPCServer := server.NewGRPCServer(
		cfg.GRPC.Addr, gRPCBenefitHandler,
	)
	log.WithField("grpc addr:", cfg.GRPC.Addr).Info("starting gRPC server")

	errCh := make(chan error, 2)

	go func() {
		errCh <- httpServer.Run()
	}()
	go func() {
		errCh <- gRPCServer.Run()
	}()

	var runErr error

	select {
	case <-signalCtx.Done():
	case runErr = <-errCh:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	grpcDone := make(chan struct{})

	go func() {
		gRPCServer.GracefulStop()
		close(grpcDone)
	}()

	httpErr := httpServer.Shutdown(shutdownCtx)

	var closeErr error
	if httpErr != nil {
		closeErr = httpServer.Close()
	}

	var timeoutErr error
	select {
	case <-grpcDone:
	case <-shutdownCtx.Done():
		gRPCServer.Stop()
		timeoutErr = shutdownCtx.Err()
	}

	return errors.Join(runErr, httpErr, closeErr, timeoutErr)

}
