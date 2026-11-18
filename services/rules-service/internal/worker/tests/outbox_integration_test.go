package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This fake implements Publisher without connecting to Kafka.
// Calls record attempts, including the one that fails.
type fakePublisher struct {
	calls  []domain.OutboxEvent
	failOn int
	err    error
}

func (p *fakePublisher) Publish(_ context.Context, event domain.OutboxEvent) error {
	p.calls = append(p.calls, event)
	if p.failOn > 0 && len(p.calls) == p.failOn {
		return p.err
	}
	return nil
}

func TestProcessBatchSuccessIntegration(t *testing.T) {
	repo, pool := newWorkerTestStorage(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Arrange: two pending events with a deterministic publication order.
	want := insertPendingEvents(t, ctx, pool)
	publisher := &fakePublisher{}
	outboxWorker := worker.NewOutboxWorker(repo, publisher, 10)

	// Act: run one batch, not a background loop.
	processed, err := outboxWorker.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("ProcessBatch(): %v", err)
	}

	// Assert: both events were handed to the publisher unchanged and in order.
	if processed != len(want) {
		t.Fatalf("processed = %d, want %d", processed, len(want))
	}
	assertPublishCalls(t, publisher.calls, want)

	// Check through an independent connection: "done" must be committed,
	// not merely visible inside the worker's transaction.
	assertEventStatuses(t, ctx, pool, want, "done")

	// A second pass must find nothing and must not publish the events again.
	// Storage has one connection, so this also catches a leaked transaction.
	processed, err = outboxWorker.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("second ProcessBatch(): %v", err)
	}
	if processed != 0 || len(publisher.calls) != len(want) {
		t.Fatalf("second pass: processed=%d, total publish calls=%d; want 0 and %d",
			processed, len(publisher.calls), len(want))
	}
}

func TestProcessBatchPublishFailureRollsBackAndRetriesIntegration(t *testing.T) {
	repo, pool := newWorkerTestStorage(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	events := insertPendingEvents(t, ctx, pool)
	publishErr := errors.New("publisher failed on the second event")
	publisher := &fakePublisher{failOn: 2, err: publishErr}
	outboxWorker := worker.NewOutboxWorker(repo, publisher, 10)

	// The first Publish succeeds, but the second one fails before commit.
	processed, err := outboxWorker.ProcessBatch(ctx)
	if !errors.Is(err, publishErr) {
		t.Fatalf("ProcessBatch() error = %v, want wrapped publisher error %v", err, publishErr)
	}
	if processed != 0 {
		t.Fatalf("processed = %d, want 0 committed events", processed)
	}
	assertPublishCalls(t, publisher.calls, events)

	// SetDone for the first event must also have been rolled back.
	// This query uses a connection outside the worker's transaction.
	assertEventStatuses(t, ctx, pool, events, "new")

	// Recover the publisher without clearing its call history.
	publisher.failOn = 0
	processed, err = outboxWorker.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("retry ProcessBatch(): %v", err)
	}
	if processed != len(events) {
		t.Fatalf("retry processed = %d, want %d", processed, len(events))
	}

	// The successfully published first event is sent again with the same ID.
	// Rolling back PostgreSQL cannot undo an external publication.
	wantCalls := []domain.OutboxEvent{events[0], events[1], events[0], events[1]}
	assertPublishCalls(t, publisher.calls, wantCalls)
	assertEventStatuses(t, ctx, pool, events, "done")

	// Once the retry commits, neither event may be selected again.
	processed, err = outboxWorker.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("ProcessBatch() after successful retry: %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed after successful retry = %d, want 0", processed)
	}
	assertPublishCalls(t, publisher.calls, wantCalls)
}

func insertPendingEvents(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []domain.OutboxEvent {
	t.Helper()
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []domain.OutboxEvent{
		{
			ID:        uuid.NewString(),
			EventType: "promotion.created",
			Payload:   json.RawMessage(`{"ID":"promotion-1","ShopID":"магазин-1"}`),
			CreatedAt: at,
		},
		{
			ID:        uuid.NewString(),
			EventType: "promotion.created",
			Payload:   json.RawMessage(`{"ID":"promotion-2","ShopID":"магазин-2"}`),
			CreatedAt: at.Add(time.Second),
		},
	}
	for _, event := range events {
		if _, err := pool.Exec(ctx, `
			INSERT INTO promotions_outbox (id, event_type, payload, created_at)
			VALUES ($1, $2, $3, $4)
		`, event.ID, event.EventType, event.Payload, event.CreatedAt); err != nil {
			t.Fatalf("insert pending event: %v", err)
		}
	}
	return events
}

func assertPublishCalls(t *testing.T, got, want []domain.OutboxEvent) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("publish calls = %d, want %d", len(got), len(want))
	}
	for i, event := range got {
		expected := want[i]
		if event.ID != expected.ID ||
			event.EventType != expected.EventType ||
			!bytes.Equal(event.Payload, expected.Payload) ||
			!event.CreatedAt.Equal(expected.CreatedAt) {
			t.Fatalf("publish attempt[%d] = %+v, want %+v", i, event, expected)
		}
	}
}

func assertEventStatuses(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	events []domain.OutboxEvent,
	want string,
) {
	t.Helper()
	for _, event := range events {
		var status string
		err := pool.QueryRow(ctx,
			"SELECT status FROM promotions_outbox WHERE id = $1", event.ID,
		).Scan(&status)
		if err != nil {
			t.Fatalf("read event status: %v", err)
		}
		if status != want {
			t.Fatalf("event %s status = %q, want %q", event.ID, status, want)
		}
	}
}
