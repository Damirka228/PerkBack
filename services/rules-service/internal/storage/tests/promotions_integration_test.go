package storage_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/config"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	storagepkg "github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func testPromotion() domain.Promotion {
	return domain.Promotion{
		ID:                   uuid.NewString(),
		ShopID:               "магазин",
		CategoryID:           "groceries",
		MinPurchaseAmountKop: 10_000,
		DiscountKop:          1_000,
		RateBPS:              500,
	}
}

func TestNewStorageIntegration(t *testing.T) {
	// Obtain an isolated, migrated schema before exercising the constructor.
	fixture := newIntegrationStorage(t)
	storage, err := storagepkg.NewStorage(storageTestContext(t), config.DB{
		DSN:            fixture.dsn,
		ConnectTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewStorage(): %v", err)
	}
	t.Cleanup(storage.Close)
	if _, err := storage.GetAllPromotions(storageTestContext(t)); err != nil {
		t.Fatalf("query initialized storage: %v", err)
	}
	storage.Close()
	storage.Close()
}

func TestGetAllPromotionsEmptyIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	promotions, err := storage.GetAllPromotions(storageTestContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if promotions == nil || len(promotions) != 0 {
		t.Fatalf("GetAllPromotions() = %v, want non-nil empty slice", promotions)
	}
}

func TestCreatePromotionAndOutboxIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	want := testPromotion()
	if err := storage.CreatePromotion(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := storage.GetAllPromotions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []domain.Promotion{want}) {
		t.Fatalf("promotions = %+v, want %+v", got, want)
	}
	var eventID, eventType, status string
	var payload []byte
	var createdAt time.Time
	err = storage.pool.QueryRow(ctx,
		"SELECT id::text, event_type, payload, status, created_at FROM promotions_outbox",
	).Scan(&eventID, &eventType, &payload, &status, &createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(eventID); err != nil || eventID == want.ID {
		t.Fatalf("event ID %q must be a separate UUID", eventID)
	}
	if eventType != "promotion.created" || status != "new" || createdAt.IsZero() {
		t.Fatalf("unexpected outbox event: type=%q, status=%q, created_at=%v", eventType, status, createdAt)
	}
	var decoded domain.Promotion
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != want {
		t.Fatalf("outbox payload = %+v, want %+v", decoded, want)
	}
}

func TestGetAllPromotionsOrderIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	first, second := testPromotion(), testPromotion()
	first.ID = "00000000-0000-0000-0000-000000000001"
	second.ID = "00000000-0000-0000-0000-000000000002"
	for _, promotion := range []domain.Promotion{second, first} {
		if err := storage.CreatePromotion(ctx, promotion); err != nil {
			t.Fatal(err)
		}
	}
	got, err := storage.GetAllPromotions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []domain.Promotion{first, second}) {
		t.Fatalf("promotions = %+v, want ID order", got)
	}
}

func TestCreatePromotionRollsBackOnOutboxErrorIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	// Inject a DB failure only in this test's empty outbox table.
	if _, err := storage.pool.Exec(ctx, `
		ALTER TABLE promotions_outbox ADD CONSTRAINT reject_test_event
		CHECK (event_type <> 'promotion.created')
	`); err != nil {
		t.Fatal(err)
	}
	err := storage.CreatePromotion(ctx, testPromotion())
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("CreatePromotion() error = %v, want check constraint violation", err)
	}
	assertStorageCounts(t, storage, 0, 0)
}

func TestCreatePromotionDuplicateIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	promotion := testPromotion()
	if err := storage.CreatePromotion(ctx, promotion); err != nil {
		t.Fatal(err)
	}
	err := storage.CreatePromotion(ctx, promotion)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate error = %v, want unique constraint violation", err)
	}
	assertStorageCounts(t, storage, 1, 1)
}

func assertStorageCounts(t *testing.T, storage *storageFixture, wantPromotions, wantEvents int) {
	t.Helper()
	var promotions, events int
	err := storage.pool.QueryRow(storageTestContext(t), `
		SELECT (SELECT count(*) FROM promotions),
		       (SELECT count(*) FROM promotions_outbox)
	`).Scan(&promotions, &events)
	if err != nil {
		t.Fatal(err)
	}
	if promotions != wantPromotions || events != wantEvents {
		t.Fatalf("row counts = (%d, %d), want (%d, %d)", promotions, events, wantPromotions, wantEvents)
	}
}
