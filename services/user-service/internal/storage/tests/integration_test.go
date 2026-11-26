package storage_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/user-service/internal/config"
	storagepkg "github.com/Damirka228/PerkBack/services/user-service/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TEST_DATABASE_URL='postgres://...' go test -race ./services/user-service/internal/storage/tests -count=1
//
// Each test applies the migration in its own random schema and drops only that
// schema during cleanup. Public tables and application migration versions stay
// untouched.
type storageFixture struct {
	*storagepkg.Storage
	pool *pgxpool.Pool // Independent connection for committed-state assertions.
}

func newIntegrationStorage(t *testing.T) *storageFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := storageTestContext(t)
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}
	t.Cleanup(admin.Close)

	schema := "user_storage_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

	scopedDSN := isolatedStorageDSN(t, dsn, schema)
	pool, err := pgxpool.New(ctx, scopedDSN)
	if err != nil {
		t.Fatalf("create isolated pool: %v", err)
	}
	t.Cleanup(pool.Close)

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration directory")
	}
	sql, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../../migrations/000001_create_user_profile.up.sql"))
	if err != nil {
		t.Fatalf("read profile migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply profile migration: %v", err)
	}

	client, err := storagepkg.NewStorage(ctx, config.DB{
		DSN: scopedDSN, ConnectTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("initialize isolated storage: %v", err)
	}
	t.Cleanup(client.Close)
	return &storageFixture{Storage: client, pool: pool}
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
	// Generated values contain no whitespace or quoting characters.
	return dsn + " search_path=" + schema + " pool_max_conns=1"
}

func storageTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}
