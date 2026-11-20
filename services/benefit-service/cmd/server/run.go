package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/client/rules"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/consumer/kafka"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/benefit"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/router"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/logger"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/server"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	ErrRuleIdEmpty = errors.New("err rule id is empty")
)

type OptionsProvider struct {
	mu      sync.RWMutex
	rateBPS int64
	rules   []domain.RulePromotion
}

func (p *OptionsProvider) Upsert(rule domain.RulePromotion) error {
	if rule.ID == "" {
		return ErrRuleIdEmpty
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, promo := range p.rules {
		if promo.ID == rule.ID {
			p.rules[i] = rule
			return nil
		}
	}
	p.rules = append(p.rules, rule)

	return nil
}

func (p *OptionsProvider) GetOptions(ctx context.Context, input usecase.CalculateInput) ([]usecase.BestVariant, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
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

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpRouter := router.NewRouter()
	httpServer := server.NewHTTPServer(
		cfg.HTTP.Addr, httpRouter, cfg.Timeout,
	)
	log.WithFields(logrus.Fields{
		"server": "http",
		"addr":   cfg.HTTP.Addr,
	}).Info("starting HTTP server")

	client, err := rules.NewClient(cfg.RulesService)
	if err != nil {
		return fmt.Errorf("err create rules clinet: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.WithError(err).Error("close rules client")
		}
	}()
	rulesPromotions, err := client.GetAllPromotionsClient(signalCtx)
	if err != nil {
		return fmt.Errorf("err to get promotions without rules client: %w", err)
	}
	log.WithField("count", len(rulesPromotions)).Info("promotions loaded")

	provider := &OptionsProvider{rateBPS: 100, rules: rulesPromotions}
	processor := kafka.NewPromotionEventProcessor(provider)
	consumer, err := kafka.NewConsumer(
		[]string{"localhost:9092"},
		"promotions.event",
		"benefit-service-local",
		processor,
	)
	if err != nil {
		return fmt.Errorf("create kafka consumer: %w", err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.WithError(err).Error("close kafka consumer")
		}
	}()

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

	errCh := make(chan error, 3)
	go func() { errCh <- httpServer.Run() }()
	go func() { errCh <- gRPCServer.Run() }()
	go func() {
		err := consumer.Run(signalCtx)
		if err != nil {
			err = fmt.Errorf("kafka consumer: %w", err)
		}
		errCh <- err
	}()

	var runErr error
	received := 0
	select {
	case <-signalCtx.Done():
		log.WithField("reason", "signal_or_context").Info("shutdown requested")
	case runErr = <-errCh:
		received++
		log.WithField("reason", "server_exit").Warn("shutdown requested")
	}
	stop()

	shutdownErr := server.Shutdown(httpServer, gRPCServer, cfg.ShutdownTimeout, log)

	if errors.Is(runErr, grpc.ErrServerStopped) {
		runErr = nil
	}
	for received < 3 {
		err := <-errCh
		received++
		if err == nil || errors.Is(err, grpc.ErrServerStopped) {
			continue
		}
		if runErr == nil {
			runErr = err
		} else {
			log.WithError(err).Error("additional server error")
		}
	}

	if runErr != nil {
		if shutdownErr != nil {
			log.WithError(shutdownErr).Error("additional shutdown error")
		}
		return runErr
	}
	return shutdownErr
}
