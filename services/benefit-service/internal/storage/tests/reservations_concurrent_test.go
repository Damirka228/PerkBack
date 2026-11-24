package storage_test

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	storagepkg "github.com/Damirka228/PerkBack/services/benefit-service/internal/storage"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func TestReserveCashbackConcurrentIntegration(t *testing.T) {
	tests := []struct {
		name         string
		orderIDs     [2]string
		wantAmounts  []int64
		wantReserved int64
	}{
		{
			name: "different_orders_share_remaining_limit", orderIDs: [2]string{"order-1", "order-2"},
			wantAmounts: []int64{1_000, 5_000}, wantReserved: 6_000,
		},
		{
			name: "same_order_is_reserved_once", orderIDs: [2]string{"order-1", "order-1"},
			wantAmounts: []int64{5_000, 5_000}, wantReserved: 5_000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			limit := monthlyLimitRow{"user-1", "2026-11-01", 100_000, 0, 94_000}
			seedMonthlyLimit(t, fixture, limit)

			// Each client owns a different single-connection pool. A shared
			// single-connection pool would serialize requests before PostgreSQL.
			var clients [2]*storagepkg.Storage
			applicationNames := make([]string, len(clients))
			for i := range clients {
				clients[i], applicationNames[i] = newConcurrentClient(t, fixture)
			}

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			blocker, err := fixture.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = blocker.Rollback(cleanupCtx)
			})
			if _, err := blocker.Exec(ctx, `
				SELECT 1 FROM cashback_monthly_limits
				WHERE user_id = $1 AND period_start = $2 FOR UPDATE
			`, limit.UserID, reservationTestPeriod()); err != nil {
				t.Fatal(err)
			}

			type result struct {
				orderID string
				amount  int64
				err     error
			}
			results := make(chan result, len(clients))
			start := make(chan struct{})
			var wg sync.WaitGroup
			t.Cleanup(func() {
				cancel()
				wg.Wait()
			})
			for i, client := range clients {
				wg.Add(1)
				go func(client *storagepkg.Storage, orderID string) {
					defer wg.Done()
					<-start
					amount, err := client.ReserveCashback(ctx, orderID, limit.UserID, reservationTestPeriod(), 5_000)
					results <- result{orderID, amount, err}
				}(client, tt.orderIDs[i])
			}
			close(start)

			// Do not guess concurrency with sleeps: both distinct backends
			// must actually be waiting on PostgreSQL locks before release.
			waitForReservationLockWaiters(t, fixture, ctx, applicationNames)
			if err := blocker.Commit(ctx); err != nil {
				t.Fatalf("release monthly limit lock: %v", err)
			}

			var amounts []int64
			saved := make(map[string]int64)
			for range clients {
				select {
				case result := <-results:
					if result.err != nil {
						t.Fatalf("%s: ReserveCashback(): %v", result.orderID, result.err)
					}
					amounts = append(amounts, result.amount)
					if previous, ok := saved[result.orderID]; ok && previous != result.amount {
						t.Fatalf("same order returned different amounts: %d and %d", previous, result.amount)
					}
					saved[result.orderID] = result.amount
				case <-ctx.Done():
					t.Fatalf("concurrent reservations did not finish: %v", ctx.Err())
				}
			}
			slices.Sort(amounts)
			if !slices.Equal(amounts, tt.wantAmounts) {
				t.Fatalf("returned amounts = %v, want %v", amounts, tt.wantAmounts)
			}
			var wantRows []reservationRow
			for orderID, amount := range saved {
				wantRows = append(wantRows, reservationRow{orderID, limit.UserID, limit.Period, amount, "reserved"})
			}
			slices.SortFunc(wantRows, func(a, b reservationRow) int {
				return strings.Compare(a.OrderID, b.OrderID)
			})
			limit.Reserved = tt.wantReserved
			assertReservationState(t, fixture, []monthlyLimitRow{limit}, wantRows)
		})
	}
}

func newConcurrentClient(t *testing.T, fixture *storageFixture) (*storagepkg.Storage, string) {
	t.Helper()
	name := "benefit_reservation_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	dsn := fixture.dsn
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		params := parsed.Query()
		params.Set("application_name", name)
		parsed.RawQuery = params.Encode()
		dsn = parsed.String()
	} else {
		dsn += " application_name=" + name
	}
	client, err := storagepkg.NewStorage(storageTestContext(t), config.DB{
		DSN: dsn, ConnectTimeout: 3 * time.Second,
	}, logrus.New())
	if err != nil {
		t.Fatalf("create concurrent storage client: %v", err)
	}
	t.Cleanup(client.Close)
	return client, name
}

func waitForReservationLockWaiters(t *testing.T, fixture *storageFixture, ctx context.Context, applicationNames []string) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		err := fixture.admin.QueryRow(ctx, `
			SELECT count(DISTINCT pid) FROM pg_stat_activity
			WHERE application_name = ANY($1::text[]) AND wait_event_type = 'Lock'
		`, applicationNames).Scan(&waiting)
		if err != nil {
			t.Fatalf("inspect concurrent PostgreSQL backends: %v", err)
		}
		if waiting == len(applicationNames) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %d distinct lock waiters (last count %d): %v",
				len(applicationNames), waiting, ctx.Err())
		case <-ticker.C:
		}
	}
}
