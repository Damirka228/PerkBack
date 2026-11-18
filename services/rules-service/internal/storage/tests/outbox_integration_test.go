package storage_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	storagepkg "github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
	"github.com/jackc/pgx/v5"
)

func insertTestEvent(t *testing.T, storage *storageFixture, number int, status string, createdAt time.Time) domain.OutboxEvent {
	t.Helper()
	event := domain.OutboxEvent{
		ID:        fmt.Sprintf("00000000-0000-0000-0000-%012d", number),
		EventType: "promotion.created",
		Payload:   json.RawMessage(`{"ID":"promotion","ShopID":"магазин"}`),
		CreatedAt: createdAt,
	}
	_, err := storage.pool.Exec(storageTestContext(t), `
		INSERT INTO promotions_outbox (id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, event.ID, event.EventType, event.Payload, status, event.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func assertEventStatus(t *testing.T, storage *storageFixture, id, want string) {
	t.Helper()
	var got string
	err := storage.pool.QueryRow(storageTestContext(t),
		"SELECT status FROM promotions_outbox WHERE id = $1", id,
	).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("event %s status = %q, want %q", id, got, want)
	}
}

func assertOutboxEvents(t *testing.T, got, want []domain.OutboxEvent) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID ||
			got[i].EventType != want[i].EventType ||
			!bytes.Equal(got[i].Payload, want[i].Payload) ||
			!got[i].CreatedAt.Equal(want[i].CreatedAt) {
			t.Fatalf("event[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBeginBatchEventsOrderFilterAndLimitIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	second := insertTestEvent(t, storage, 2, "new", at)
	first := insertTestEvent(t, storage, 1, "new", at)
	// The later event has a smaller ID: created_at must take precedence.
	insertTestEvent(t, storage, 0, "new", at.Add(time.Second))
	insertTestEvent(t, storage, 4, "done", at.Add(-time.Second))
	batch := beginTestBatch(t, storage, 2)
	assertOutboxEvents(t, batch.GetEvents(), []domain.OutboxEvent{first, second})
}

func TestEmptyOutboxBatchIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	batch := beginTestBatch(t, storage, 10)
	if got := batch.GetEvents(); got == nil || len(got) != 0 {
		t.Fatalf("events = %v, want non-nil empty slice", got)
	}
	if err := batch.CommitTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	assertStorageConnectionReleased(t, storage)
}

func TestOutboxBatchCommitIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := insertTestEvent(t, storage, 1, "new", at)
	second := insertTestEvent(t, storage, 2, "new", at)
	batch := beginTestBatch(t, storage, 2)
	// Exercise membership lookup beyond the first event.
	if err := batch.SetDone(storageTestContext(t), second.ID); err != nil {
		t.Fatal(err)
	}
	// A separate connection must not see an uncommitted status update.
	assertEventStatus(t, storage, second.ID, "new")
	if err := batch.CommitTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	assertEventStatus(t, storage, first.ID, "new")
	assertEventStatus(t, storage, second.ID, "done")
	next := beginTestBatch(t, storage, 10)
	assertOutboxEvents(t, next.GetEvents(), []domain.OutboxEvent{first})
	if err := batch.RollbackTX(storageTestContext(t)); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("rollback after commit = %v, want ErrTxClosed", err)
	}
}

func TestOutboxBatchRollbackIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	event := insertTestEvent(t, storage, 1, "new", time.Now().UTC().Truncate(time.Second))
	batch := beginTestBatch(t, storage, 1)
	if err := batch.SetDone(storageTestContext(t), event.ID); err != nil {
		t.Fatal(err)
	}
	if err := batch.RollbackTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	assertEventStatus(t, storage, event.ID, "new")
	retry := beginTestBatch(t, storage, 1)
	assertOutboxEvents(t, retry.GetEvents(), []domain.OutboxEvent{event})
}

func TestOutboxBatchesSkipLockedIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var events []domain.OutboxEvent
	for i := 1; i <= 4; i++ {
		events = append(events, insertTestEvent(t, storage, i, "new", at))
	}
	// Transactions overlap on distinct connections, just as two workers would.
	first := beginTestBatch(t, storage, 2)
	second := beginTestBatch(t, storage.newClient(t), 2)
	assertOutboxEvents(t, first.GetEvents(), events[:2])
	assertOutboxEvents(t, second.GetEvents(), events[2:])
	empty := beginTestBatch(t, storage.newClient(t), 10)
	if len(empty.GetEvents()) != 0 {
		t.Fatalf("all events are locked, got %+v", empty.GetEvents())
	}
	if err := empty.RollbackTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	if err := first.RollbackTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	retry := beginTestBatch(t, storage, 2)
	assertOutboxEvents(t, retry.GetEvents(), events[:2])
}

func TestOutboxBatchSetDoneForeignIDIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insertTestEvent(t, storage, 1, "new", at)
	foreign := insertTestEvent(t, storage, 2, "new", at)
	batch := beginTestBatch(t, storage, 1)
	if err := batch.SetDone(storageTestContext(t), foreign.ID); !errors.Is(err, storagepkg.ErrNotEventsID) {
		t.Fatalf("SetDone(foreign) = %v, want ErrNotEventsID", err)
	}
	if err := batch.CommitTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	assertEventStatus(t, storage, foreign.ID, "new")
}

func TestOutboxBatchSetDoneTwiceIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	event := insertTestEvent(t, storage, 1, "new", time.Now().UTC().Truncate(time.Second))
	batch := beginTestBatch(t, storage, 1)
	if err := batch.SetDone(storageTestContext(t), event.ID); err != nil {
		t.Fatal(err)
	}
	if err := batch.SetDone(storageTestContext(t), event.ID); !errors.Is(err, storagepkg.ErrNotFound) {
		t.Fatalf("second SetDone() = %v, want ErrNotFound", err)
	}
	if err := batch.CommitTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	assertEventStatus(t, storage, event.ID, "done")
}

func TestBeginBatchEventsQueryErrorReleasesConnectionIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	// Only drop the table in this test's disposable schema.
	if _, err := storage.pool.Exec(storageTestContext(t), "DROP TABLE promotions_outbox"); err != nil {
		t.Fatal(err)
	}
	batch, err := storage.BeginBatchEvents(storageTestContext(t), 1)
	if err == nil || batch != nil {
		t.Fatalf("missing table: batch=%v, error=%v", batch, err)
	}
	assertStorageConnectionReleased(t, storage)
}

func TestBeginBatchEventsScanErrorReleasesConnectionIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	// Deliberately incompatible data exercises Scan cleanup without changing
	// application migrations or any shared table.
	if _, err := storage.pool.Exec(ctx,
		"ALTER TABLE promotions_outbox ALTER COLUMN created_at TYPE text USING created_at::text",
	); err != nil {
		t.Fatal(err)
	}
	insertTestEvent(t, storage, 1, "new", time.Now().UTC().Truncate(time.Second))
	if _, err := storage.pool.Exec(ctx, "UPDATE promotions_outbox SET created_at = 'invalid timestamp'"); err != nil {
		t.Fatal(err)
	}
	batch, err := storage.BeginBatchEvents(ctx, 1)
	if err == nil || batch != nil {
		t.Fatalf("invalid timestamp: batch=%v, error=%v", batch, err)
	}
	assertStorageConnectionReleased(t, storage)
}

func TestOutboxBatchFinishTransactionIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	for _, operation := range []string{"commit", "rollback"} {
		t.Run(operation, func(t *testing.T) {
			batch := beginTestBatch(t, storage, 1)
			ctx := storageTestContext(t)
			finish := batch.CommitTX
			if operation == "rollback" {
				finish = batch.RollbackTX
			}
			if err := finish(ctx); err != nil {
				t.Fatalf("first %s: %v", operation, err)
			}
			if err := finish(ctx); !errors.Is(err, pgx.ErrTxClosed) {
				t.Fatalf("repeated %s = %v, want ErrTxClosed", operation, err)
			}
		})
	}
}

func TestOutboxBatchSetDoneQueryErrorIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	event := insertTestEvent(t, storage, 1, "new", time.Now().UTC().Truncate(time.Second))
	batch := beginTestBatch(t, storage, 1)
	if err := batch.RollbackTX(storageTestContext(t)); err != nil {
		t.Fatal(err)
	}
	if err := batch.SetDone(storageTestContext(t), event.ID); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("SetDone() on a closed transaction = %v, want ErrTxClosed", err)
	}
	assertEventStatus(t, storage, event.ID, "new")
}
