package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestGetReservByOrderIDReadsExactOrderWithoutWritesIntegration(t *testing.T) {
	for _, status := range []string{"reserved", "credited", "canceled"} {
		t.Run(status, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			ctx := storageTestContext(t)
			// Multiple months and orders for one user must not be confused.
			limits := []monthlyLimitRow{
				{"user-1", "2026-01-01", 100_000, 0, 0},
				{"user-1", "2026-02-01", 100_000, 5_000, 0},
			}
			if status == "reserved" {
				limits[0].Reserved = 2_000
			} else if status == "credited" {
				limits[0].Credited = 2_000
			}
			for _, limit := range limits {
				seedMonthlyLimit(t, fixture, limit)
			}
			reservations := []reservationRow{
				{"order-1", "user-1", "2026-01-01", 2_000, status},
				{"order-2", "user-1", "2026-02-01", 5_000, "reserved"},
				{"order-3", "user-1", "2026-02-01", 0, "reserved"},
			}
			for _, row := range reservations {
				if _, err := fixture.pool.Exec(ctx, `
					INSERT INTO cashback_reservations
						(order_id, user_id, period_start, amount_kopecks, status)
					VALUES ($1, $2, $3::date, $4, $5)
				`, row.OrderID, row.UserID, row.Period, row.Amount, row.Status); err != nil {
					t.Fatalf("seed reservation: %v", err)
				}
			}

			for _, want := range reservations {
				for range 2 {
					got, err := fixture.GetReservByOrderID(ctx, want.OrderID)
					if err != nil {
						t.Fatalf("GetReservByOrderID(%q): %v", want.OrderID, err)
					}
					if got.UserID != want.UserID || got.PeriodStart.Format(time.DateOnly) != want.Period ||
						got.AmountKop != want.Amount || got.Status != want.Status {
						t.Errorf("GetReservByOrderID(%q) = %+v, want %+v", want.OrderID, got, want)
					}
				}
			}
			got, err := fixture.GetReservByOrderID(ctx, "missing-order")
			if got != (domain.CashbackReservation{}) || !errors.Is(err, domain.ErrReservationNotFound) {
				t.Fatalf("missing order = (%+v, %v), want empty result and ErrReservationNotFound", got, err)
			}
			assertReservationState(t, fixture, limits, reservations)
		})
	}
}

func TestGetReservByOrderIDMissingDoesNotCreateRowsIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	got, err := fixture.GetReservByOrderID(storageTestContext(t), "missing-order")
	if got != (domain.CashbackReservation{}) || !errors.Is(err, domain.ErrReservationNotFound) {
		t.Fatalf("GetReservByOrderID() = (%+v, %v), want empty result and ErrReservationNotFound", got, err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Error("lookup leaked pgx.ErrNoRows instead of the domain error")
	}
	assertReservationState(t, fixture, nil, nil)
}

func TestGetReservByOrderIDPreservesDatabaseErrorIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	// Break only the isolated schema's SELECT, without changing production SQL.
	if _, err := fixture.pool.Exec(ctx, `
		ALTER TABLE cashback_reservations RENAME COLUMN status TO saved_status
	`); err != nil {
		t.Fatal(err)
	}
	got, err := fixture.GetReservByOrderID(ctx, "order-1")
	var pgErr *pgconn.PgError
	if got != (domain.CashbackReservation{}) || !errors.As(err, &pgErr) || pgErr.Code != "42703" {
		t.Fatalf("GetReservByOrderID() = (%+v, %v), want empty result and undefined-column error", got, err)
	}
	if errors.Is(err, domain.ErrReservationNotFound) {
		t.Error("a database failure must not be reported as a missing reservation")
	}
}

func TestGetReservByOrderIDCanceledContextIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := fixture.GetReservByOrderID(ctx, "order-1")
	if got != (domain.CashbackReservation{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("GetReservByOrderID() = (%+v, %v), want empty result and context.Canceled", got, err)
	}
	if errors.Is(err, domain.ErrReservationNotFound) {
		t.Error("context cancellation must not be reported as a missing reservation")
	}
	assertReservationState(t, fixture, nil, nil)
}
