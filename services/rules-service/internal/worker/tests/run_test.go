package worker_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/worker"
	"github.com/sirupsen/logrus"
)

func newWorkerTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

func TestRunInvalidArguments(t *testing.T) {
	log := newWorkerTestLogger()
	tests := []struct {
		name          string
		pollInterval  time.Duration
		retryInterval time.Duration
		log           *logrus.Logger
	}{
		{name: "zero_poll_interval", retryInterval: time.Second, log: log},
		{name: "negative_poll_interval", pollInterval: -time.Second, retryInterval: time.Second, log: log},
		{name: "zero_retry_interval", pollInterval: time.Second, log: log},
		{name: "negative_retry_interval", pollInterval: time.Second, retryInterval: -time.Second, log: log},
		{name: "nil_logger", pollInterval: time.Second, retryInterval: time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No storage or publisher: validation must happen before processing.
			outboxWorker := worker.NewOutboxWorker(nil, nil, 10)
			if err := outboxWorker.Run(t.Context(), tt.pollInterval, tt.retryInterval, tt.log); err == nil {
				t.Fatal("Run() must reject invalid arguments")
			}
		})
	}
}

func TestRunCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	outboxWorker := worker.NewOutboxWorker(nil, nil, 10)
	if err := outboxWorker.Run(ctx, time.Hour, time.Hour, newWorkerTestLogger()); err != nil {
		t.Fatalf("Run() with a canceled context = %v, want nil", err)
	}
}
