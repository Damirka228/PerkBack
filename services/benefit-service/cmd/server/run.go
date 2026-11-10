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
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/benefit"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/router"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/logger"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/server"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"github.com/sirupsen/logrus"
)

type OptionsProvider struct {
	rateBPS int64
	rules   []domain.RulePromotion
}

func (p *OptionsProvider) GetOptions(ctx context.Context, input usecase.CalculateInput) ([]usecase.BestVariant, error) {
	bestVatiant := []usecase.BestVariant{
		{DiscountKop: 0, RateBPS: p.rateBPS},
	}
	for _, rule := range p.rules {
		matches := rule.MatchesRule(input.ShopID, input.CategoryID, input.PurchaseAmountKop)
		if matches == true {
			bestVatiant = append(bestVatiant, usecase.BestVariant{
				DiscountKop: rule.DiscountKop,
				RateBPS:     rule.RateBPS,
			})
		}
	}
	return bestVatiant, nil
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

	_, httpAddrFromEnv := os.LookupEnv("BENEFIT_HTTP_ADDR")
	log.WithFields(logrus.Fields{
		"http_addr":          cfg.HTTP.Addr,
		"grpc_addr":          cfg.GRPC.Addr,
		"http_addr_from_env": httpAddrFromEnv,
	}).Debug("server addresses loaded")

	log.WithFields(logrus.Fields{
		"config_path": configPath,
		"app":         cfg.App.Name,
		"env":         cfg.App.Env,
		"log_level":   cfg.Log.Level,
	}).Info("configuration loaded successfully")

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

	testHardRules := []domain.RulePromotion{
		{
			ShopID:               "shop-42",
			CategoryID:           "food",
			MinPurchaseAmountKop: 100_000,
			DiscountKop:          7_000,
			RateBPS:              300,
		},
	}

	provider := &OptionsProvider{rateBPS: 100, rules: testHardRules}
	service := usecase.NewService(provider)
	gRPCBenefitHandler := benefit.NewHandler(service)
	gRPCServer := server.NewGRPCServer(
		cfg.GRPC.Addr,
		gRPCBenefitHandler,
		log,
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
