package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
	"github.com/sirupsen/logrus"
)

type Publisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
}

type OutboxWorker struct {
	repo       *storage.Storage
	publisher  Publisher
	limitBatch int
}

func NewOutboxWorker(repo *storage.Storage, publisher Publisher, limitBatch int) *OutboxWorker {
	return &OutboxWorker{
		repo:       repo,
		publisher:  publisher,
		limitBatch: limitBatch,
	}
}

func (w *OutboxWorker) ProcessBatch(ctx context.Context) (int, error) {
	batch, err := w.repo.BeginBatchEvents(ctx, w.limitBatch)
	if err != nil {
		return 0, fmt.Errorf("err batch events:%w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()
		_ = batch.RollbackTX(rollbackCtx)
	}()

	events := batch.GetEvents()
	for _, event := range events {
		if err := w.publisher.Publish(ctx, event); err != nil {
			return 0, fmt.Errorf("err publish:%w", err)
		}
		if err := batch.SetDone(ctx, event.ID); err != nil {
			return 0, fmt.Errorf("err set done batch:%w", err)
		}
	}
	if err := batch.CommitTX(ctx); err != nil {
		return 0, fmt.Errorf("err commit txc:%w", err)
	}

	return len(events), nil
}

func (w *OutboxWorker) Run(ctx context.Context, pollInterval time.Duration, retryInterval time.Duration, log *logrus.Logger) error {
	if pollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	if retryInterval <= 0 {
		return fmt.Errorf("retry interval must be positive")
	}
	if log == nil {
		return fmt.Errorf("logger must not be nil")
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		processed, err := w.ProcessBatch(ctx)
		if ctx.Err() != nil {
			return nil
		}

		delay := pollInterval

		if err != nil {
			log.WithError(err).Error("outbox batch failed")
			delay = retryInterval
		} else if processed > 0 {
			log.WithField("processed", processed).Info("outbox batch completed")
			continue
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:

		}
	}
}
