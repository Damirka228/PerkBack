package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotEventsID = errors.New("not eventID")
	ErrNotFound    = errors.New("rows not foud")
)

type OutboxBatch struct {
	tx     pgx.Tx
	events []domain.OutboxEvent
}

func (s *Storage) BeginBatchEvents(ctx context.Context, limit int) (*OutboxBatch, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit <=0")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("err to start transactions:%w", err)
	}

	handedOff := false

	defer func() {
		if handedOff {
			return
		}

		rollbackCtx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	sqlQuery := `
		SELECT id::text, event_type, payload, created_at
		FROM promotions_outbox
		WHERE status = 'new'
		ORDER BY created_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED;
	`
	rows, err := tx.Query(ctx, sqlQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("err to sql query:%w", err)
	}
	defer rows.Close()
	events := make([]domain.OutboxEvent, 0)
	for rows.Next() {
		var event domain.OutboxEvent
		if err := rows.Scan(&event.ID, &event.EventType, &event.Payload, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("err to get rows:%w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("err rows:%w", err)
	}
	rows.Close()
	handedOff = true
	return &OutboxBatch{
		tx:     tx,
		events: events,
	}, nil
}

func (b *OutboxBatch) GetEvents() []domain.OutboxEvent {
	return b.events
}

func (b *OutboxBatch) CommitTX(ctx context.Context) error {
	if err := b.tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit outbox batch: %w", err)
	}
	return nil
}

func (b *OutboxBatch) RollbackTX(ctx context.Context) error {
	if err := b.tx.Rollback(ctx); err != nil {
		return fmt.Errorf("rollback outbox batch:%w", err)
	}
	return nil
}

func (b *OutboxBatch) SetDone(ctx context.Context, eventID string) error {
	found := false
	for _, event := range b.events {
		if event.ID == eventID {
			found = true
			break
		}
	}
	if !found {
		return ErrNotEventsID
	}
	sqlQuery := `
		UPDATE promotions_outbox 
		SET status = 'done'
		WHERE id = $1 AND status = 'new';
	`
	tag, err := b.tx.Exec(ctx, sqlQuery, eventID)
	if err != nil {
		return fmt.Errorf("err sql query exec:%w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}
