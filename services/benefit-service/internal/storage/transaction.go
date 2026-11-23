package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
)

func (s *Storage) transaction(ctx context.Context, log *logrus.Logger, f func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction begin: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := tx.Rollback(rollbackCtx); err != nil &&
			!errors.Is(err, pgx.ErrTxClosed) {
			log.WithError(err).Error("transaction rollback failed")
		}
	}()

	if err := f(tx); err != nil {
		return fmt.Errorf("transaction operation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction fail: %w", err)
	}

	return nil
}
