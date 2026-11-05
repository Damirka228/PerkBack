package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/benefit"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/router"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/logger"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/server"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"github.com/sirupsen/logrus"
)

type RateProvider struct {
	rateBPS int64
}

func (p *RateProvider) GetRateBPS(ctx context.Context, input usecase.CalculateInput) (int64, error) {
	return p.rateBPS, nil
}

func run(ctx context.Context, configPath string) error {
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
	log.WithFields(logrus.Fields{
		"server": "http",
		"addr":   cfg.HTTP.Addr,
	}).Info("starting HTTP server")

	provider := &RateProvider{rateBPS: 100}
	service := usecase.NewService(provider)
	gRPCBenefitHandler := benefit.NewHandler(service)
	gRPCServer := server.NewGRPCServer(
		cfg.GRPC.Addr, gRPCBenefitHandler,
	)
	log.WithFields(logrus.Fields{
		"server": "grpc",
		"addr":   cfg.GRPC.Addr,
	}).Info("starting gRPC server")

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
		log.WithField("reason", "signal").Info("shutdown requested")
	case runErr = <-errCh:
		log.WithField("reason", "server_exit").Warn("shutdown requested")
	}

	shutdownStarted := time.Now()
	log.WithField("timeout", cfg.ShutdownTimeout.String()).Info("starting graceful shutdown")

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
		log.WithField("server", "http").Warn("graceful shutdown failed; forcing connection close")
		closeErr = httpServer.Close()
		if closeErr == nil {
			log.WithField("server", "http").Info("server connections closed")
		}
	} else {
		log.WithField("server", "http").Info("server stopped gracefully")
	}

	var timeoutErr error
	select {
	case <-grpcDone:
		log.WithField("server", "grpc").Info("server stopped gracefully")
	case <-shutdownCtx.Done():
		log.WithField("server", "grpc").Warn("shutdown deadline reached; forcing stop")
		gRPCServer.Stop()
		timeoutErr = shutdownCtx.Err()
	}

	shutdownLog := log.WithField("duration", time.Since(shutdownStarted).String())
	if httpErr != nil || closeErr != nil || timeoutErr != nil {
		shutdownLog.Warn("shutdown finished with errors")
	} else {
		shutdownLog.Info("graceful shutdown completed")
	}

	return errors.Join(runErr, httpErr, closeErr, timeoutErr)

}
