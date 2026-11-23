package storage_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Keep this contract in the test so the suite compiles before the method is
// implemented. With PostgreSQL enabled, a missing method is a failure, not a skip.
type cashbackReserver interface {
	ReserveCashback(
		ctx context.Context,
		orderID, userID string,
		periodStart time.Time,
		requestedKopecks int64,
	) (int64, error)
}

func TestReserveCashbackSharesRemainingLimitAndIsIdempotentIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	const userID = "user-1"
	periodStart := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)

	_, err := fixture.pool.Exec(ctx, `
		INSERT INTO cashback_monthly_limits
			(user_id, period_start, limit_kopecks, reserved_kopecks, credited_kopecks)
		VALUES ($1, $2, 100000, 0, 94000)
	`, userID, periodStart)
	if err != nil {
		t.Fatalf("seed monthly limit: %v", err)
	}

	reserver, ok := any(fixture.Storage).(cashbackReserver)
	if !ok {
		t.Fatal("Storage must implement ReserveCashback(context.Context, string, string, time.Time, int64) (int64, error)")
	}

	type reservationRow struct {
		orderID     string
		userID      string
		periodStart string
		amount      int64
		status      string
	}
	first := reservationRow{"order-1", userID, "2026-11-01", 5_000, "reserved"}
	second := reservationRow{"order-2", userID, "2026-11-01", 1_000, "reserved"}

	steps := []struct {
		name         string
		orderID      string
		wantAmount   int64
		wantReserved int64
		wantRows     []reservationRow
	}{
		{
			name: "first_order_reserves_requested_amount", orderID: "order-1",
			wantAmount: 5_000, wantReserved: 5_000,
			wantRows: []reservationRow{first},
		},
		{
			name: "second_order_gets_only_remaining_limit", orderID: "order-2",
			wantAmount: 1_000, wantReserved: 6_000,
			wantRows: []reservationRow{first, second},
		},
		{
			name: "repeat_returns_original_amount_even_when_limit_is_exhausted", orderID: "order-2",
			wantAmount: 1_000, wantReserved: 6_000,
			wantRows: []reservationRow{first, second},
		},
	}
	// Steps deliberately share state and run sequentially.
	for _, step := range steps {
		got, err := reserver.ReserveCashback(ctx, step.orderID, userID, periodStart, 5_000)
		if err != nil {
			t.Fatalf("%s: ReserveCashback(): %v", step.name, err)
		}
		if got != step.wantAmount {
			t.Fatalf("%s: reserved amount = %d, want %d", step.name, got, step.wantAmount)
		}

		// Read through an independent pool: uncommitted changes must not count
		// as success, and reserving must never change credited cashback.
		var limit, reserved, credited int64
		err = fixture.pool.QueryRow(ctx, `
			SELECT limit_kopecks, reserved_kopecks, credited_kopecks
			FROM cashback_monthly_limits
			WHERE user_id = $1 AND period_start = $2
		`, userID, periodStart).Scan(&limit, &reserved, &credited)
		if err != nil {
			t.Fatalf("%s: read monthly limit: %v", step.name, err)
		}
		if limit != 100_000 || reserved != step.wantReserved || credited != 94_000 {
			t.Fatalf("%s: monthly amounts = (limit=%d, reserved=%d, credited=%d), want (100000, %d, 94000)",
				step.name, limit, reserved, credited, step.wantReserved)
		}

		rows, err := fixture.pool.Query(ctx, `
			SELECT order_id, user_id, period_start::text, amount_kopecks, status
			FROM cashback_reservations
			ORDER BY order_id
		`)
		if err != nil {
			t.Fatalf("%s: query reservations: %v", step.name, err)
		}
		gotRows, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (reservationRow, error) {
			var reservation reservationRow
			err := row.Scan(&reservation.orderID, &reservation.userID, &reservation.periodStart,
				&reservation.amount, &reservation.status)
			return reservation, err
		})
		if err != nil {
			t.Fatalf("%s: read reservations: %v", step.name, err)
		}
		if !slices.Equal(gotRows, step.wantRows) {
			t.Fatalf("%s: reservations = %+v, want %+v", step.name, gotRows, step.wantRows)
		}
	}
}
