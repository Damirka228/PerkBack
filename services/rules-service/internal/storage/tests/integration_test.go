package storage_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/config"
	storagepkg "github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run PostgreSQL tests with:
// TEST_DATABASE_URL='postgres://...' go test ./services/rules-service/internal/storage/tests -count=1
//
// Each test creates and drops only its own random schema, applying the existing
// migrations there. No public tables or application records are modified.
type storageFixture struct {
	*storagepkg.Storage
	pool *pgxpool.Pool // Independent connection for fixtures and assertions.
	dsn  string
}

func newIntegrationStorage(t *testing.T) *storageFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := storageTestContext(t)
	adminConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test database config: %v", err)
	}
	adminConfig.MaxConns = 2
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}
	t.Cleanup(admin.Close)

	schema := "storage_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatalf("create isolated test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("drop isolated test schema: %v", err)
		}
	})

	// The storage under test has one connection: a leaked transaction makes the
	// next public method time out instead of hiding behind a spare connection.
	scopedDSN := isolatedStorageDSN(t, dsn, schema)
	poolConfig, err := pgxpool.ParseConfig(scopedDSN)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("create isolated pool: %v", err)
	}
	t.Cleanup(pool.Close)

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration directory")
	}
	for _, name := range []string{
		"000001_create_promotions.up.sql",
		"000002_create_promotions_outbox.up.sql",
	} {
		sql, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../../migrations", name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
	fixture := &storageFixture{pool: pool, dsn: scopedDSN}
	fixture.Storage = fixture.newClient(t).Storage
	return fixture
}

func isolatedStorageDSN(t *testing.T, dsn, schema string) string {
	t.Helper()
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("parse test database URL: %v", err)
		}
		params := parsed.Query()
		params.Set("search_path", schema)
		params.Set("pool_max_conns", "1")
		parsed.RawQuery = params.Encode()
		return parsed.String()
	}
	// pgx also accepts keyword/value DSNs. Both appended values are generated
	// by this fixture and contain no whitespace or quoting characters.
	return dsn + " search_path=" + schema + " pool_max_conns=1"
}

func (fixture *storageFixture) newClient(t *testing.T) *storageFixture {
	t.Helper()
	client, err := storagepkg.NewStorage(storageTestContext(t), config.DB{
		DSN:            fixture.dsn,
		ConnectTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("initialize isolated storage: %v", err)
	}
	t.Cleanup(client.Close)
	return &storageFixture{Storage: client, pool: fixture.pool, dsn: fixture.dsn}
}

func assertStorageConnectionReleased(t *testing.T, fixture *storageFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := fixture.GetAllPromotions(ctx); err != nil {
		t.Fatalf("single-connection storage is not usable after finishing the batch: %v", err)
	}
}

func storageTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func beginTestBatch(t *testing.T, storage *storageFixture, limit int) *storagepkg.OutboxBatch {
	t.Helper()
	batch, err := storage.BeginBatchEvents(storageTestContext(t), limit)
	if err != nil {
		t.Fatalf("begin outbox batch: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := batch.RollbackTX(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("clean up outbox batch: %v", err)
		}
	})
	return batch
}

func TestStorageCanceledContextIntegration(t *testing.T) {
	storage := newIntegrationStorage(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	t.Run("begin_batch", func(t *testing.T) {
		batch, err := storage.BeginBatchEvents(ctx, 1)
		if batch != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("BeginBatchEvents() = (%v, %v), want nil and context.Canceled", batch, err)
		}
	})
	t.Run("get_promotions", func(t *testing.T) {
		promotions, err := storage.GetAllPromotions(ctx)
		if promotions != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("GetAllPromotions() = (%v, %v), want nil and context.Canceled", promotions, err)
		}
	})
	t.Run("create_promotion", func(t *testing.T) {
		if err := storage.CreatePromotion(ctx, testPromotion()); !errors.Is(err, context.Canceled) {
			t.Fatalf("CreatePromotion() error = %v, want context.Canceled", err)
		}
	})
	t.Run("initialize_storage", func(t *testing.T) {
		got, err := storagepkg.NewStorage(ctx, config.DB{
			DSN:            os.Getenv("TEST_DATABASE_URL"),
			ConnectTimeout: time.Second,
		})
		if got != nil {
			got.Close()
			t.Fatal("NewStorage() returned storage for an already canceled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("NewStorage() error = %v, want context.Canceled", err)
		}
	})
	assertStorageCounts(t, storage, 0, 0)
}
