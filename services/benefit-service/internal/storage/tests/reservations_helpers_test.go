package storage_test

import (
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type monthlyLimitRow struct {
	UserID   string
	Period   string
	Limit    int64
	Reserved int64
	Credited int64
}

type reservationRow struct {
	OrderID string
	UserID  string
	Period  string
	Amount  int64
	Status  string
}

func reservationTestPeriod() time.Time {
	return time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
}

func seedMonthlyLimit(t *testing.T, fixture *storageFixture, limit monthlyLimitRow) {
	t.Helper()
	_, err := fixture.pool.Exec(storageTestContext(t), `
		INSERT INTO cashback_monthly_limits
			(user_id, period_start, limit_kopecks, reserved_kopecks, credited_kopecks)
		VALUES ($1, $2::date, $3, $4, $5)
	`, limit.UserID, limit.Period, limit.Limit, limit.Reserved, limit.Credited)
	if err != nil {
		t.Fatalf("seed monthly limit: %v", err)
	}
}

func assertReservationState(t *testing.T, fixture *storageFixture, wantLimits []monthlyLimitRow, wantReservations []reservationRow) {
	t.Helper()
	ctx := storageTestContext(t)
	// This pool is independent of Storage: only committed changes are visible.
	rows, err := fixture.pool.Query(ctx, `
		SELECT user_id, period_start::text, limit_kopecks, reserved_kopecks, credited_kopecks
		FROM cashback_monthly_limits ORDER BY user_id, period_start
	`)
	if err != nil {
		t.Fatalf("query monthly limits: %v", err)
	}
	limits, err := pgx.CollectRows(rows, pgx.RowToStructByPos[monthlyLimitRow])
	if err != nil {
		t.Fatalf("read monthly limits: %v", err)
	}
	if !slices.Equal(limits, wantLimits) {
		t.Fatalf("monthly limits = %+v, want %+v", limits, wantLimits)
	}

	rows, err = fixture.pool.Query(ctx, `
		SELECT order_id, user_id, period_start::text, amount_kopecks, status
		FROM cashback_reservations ORDER BY order_id
	`)
	if err != nil {
		t.Fatalf("query reservations: %v", err)
	}
	reservations, err := pgx.CollectRows(rows, pgx.RowToStructByPos[reservationRow])
	if err != nil {
		t.Fatalf("read reservations: %v", err)
	}
	if !slices.Equal(reservations, wantReservations) {
		t.Fatalf("reservations = %+v, want %+v", reservations, wantReservations)
	}
}
