package storage_test

import (
	"context"
	"errors"
	"testing"

	storagepkg "github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
)

func TestBeginBatchEventsInvalidLimit(t *testing.T) {
	for _, limit := range []int{0, -1} {
		// No pool: invalid arguments must be rejected before touching the DB.
		storage := &storagepkg.Storage{}
		batch, err := storage.BeginBatchEvents(context.Background(), limit)
		if err == nil || batch != nil {
			t.Fatalf("BeginBatchEvents(limit=%d) = (%v, %v), want nil batch and an error", limit, batch, err)
		}
	}
}

func TestOutboxBatchGetEvents(t *testing.T) {
	batch := &storagepkg.OutboxBatch{}
	if got := batch.GetEvents(); got != nil {
		t.Fatalf("GetEvents() = %v, want nil for a zero-value batch", got)
	}
}

func TestOutboxBatchSetDoneRejectsForeignID(t *testing.T) {
	batch := &storagepkg.OutboxBatch{}
	if err := batch.SetDone(t.Context(), "foreign"); !errors.Is(err, storagepkg.ErrNotEventsID) {
		t.Fatalf("SetDone() error = %v, want %v", err, storagepkg.ErrNotEventsID)
	}
}
