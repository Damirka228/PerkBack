package worker_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/config"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TEST_DATABASE_URL='postgres://...' go test ./services/rules-service/internal/worker/tests -count=1
//
// The worker uses a real storage instance, but only in a disposable schema.
// Application tables and records are never modified.
func newWorkerTestStorage(t *testing.T) (*storage.Storage, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	adminConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse database config: %v", err)
	}
	adminConfig.MaxConns = 2
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("create admin pool: %v", err)
	}
	t.Cleanup(admin.Close)

	schema := "worker_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
	})

	// Set search_path in the DSN so both connections use only the test schema.
	scopedDSN := dsn + " search_path=" + schema + " pool_max_conns=1"
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("parse database URL: %v", err)
		}
		params := parsed.Query()
		params.Set("search_path", schema)
		params.Set("pool_max_conns", "1")
		parsed.RawQuery = params.Encode()
		scopedDSN = parsed.String()
	}

	poolConfig, err := pgxpool.ParseConfig(scopedDSN)
	if err != nil {
		t.Fatalf("parse isolated database config: %v", err)
	}
	poolConfig.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("create fixture pool: %v", err)
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

	repo, err := storage.NewStorage(ctx, config.DB{
		DSN:            scopedDSN,
		ConnectTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	t.Cleanup(repo.Close)
	return repo, pool
}
