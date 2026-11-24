package storage_test

import (
	"errors"
	"testing"

	storagepkg "github.com/Damirka228/PerkBack/services/benefit-service/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestReserveCashbackCreatesMonthlyLimitIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	amount, err := fixture.ReserveCashback(storageTestContext(t), "order-1", "user-1", reservationTestPeriod(), 5_000)
	if err != nil || amount != 5_000 {
		t.Fatalf("ReserveCashback() = (%d, %v), want (5000, nil)", amount, err)
	}
	assertReservationState(t, fixture,
		[]monthlyLimitRow{{"user-1", "2026-11-01", 100_000, 5_000, 0}},
		[]reservationRow{{"order-1", "user-1", "2026-11-01", 5_000, "reserved"}},
	)
}

func TestReserveCashbackZeroAmountIsSavedAndReplayedIntegration(t *testing.T) {
	for _, tt := range []struct {
		name      string
		credited  int64
		requested int64
	}{
		{name: "exhausted_limit", credited: 100_000, requested: 5_000},
		{name: "zero_requested", credited: 94_000, requested: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			limit := monthlyLimitRow{"user-1", "2026-11-01", 100_000, 0, tt.credited}
			seedMonthlyLimit(t, fixture, limit)
			ctx := storageTestContext(t)
			amount, err := fixture.ReserveCashback(ctx, "order-1", limit.UserID, reservationTestPeriod(), tt.requested)
			if err != nil || amount != 0 {
				t.Fatalf("ReserveCashback() = (%d, %v), want (0, nil)", amount, err)
			}
			wantRows := []reservationRow{{"order-1", limit.UserID, limit.Period, 0, "reserved"}}
			assertReservationState(t, fixture, []monthlyLimitRow{limit}, wantRows)

			// Even if capacity becomes available and the retry asks for more,
			// the original order must retain its stored zero amount.
			if _, err := fixture.pool.Exec(ctx, `
				UPDATE cashback_monthly_limits SET credited_kopecks = 90000
				WHERE user_id = $1 AND period_start = $2
			`, limit.UserID, reservationTestPeriod()); err != nil {
				t.Fatal(err)
			}
			limit.Credited = 90_000
			amount, err = fixture.ReserveCashback(ctx, "order-1", limit.UserID, reservationTestPeriod(), 5_000)
			if err != nil || amount != 0 {
				t.Fatalf("replayed ReserveCashback() = (%d, %v), want (0, nil)", amount, err)
			}
			assertReservationState(t, fixture, []monthlyLimitRow{limit}, wantRows)
		})
	}
}

func TestReserveCashbackNegativeAmountDoesNotWriteIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	amount, err := fixture.ReserveCashback(storageTestContext(t), "order-1", "user-1", reservationTestPeriod(), -1)
	if amount != 0 || !errors.Is(err, storagepkg.ErrNegativeKopecks) {
		t.Fatalf("ReserveCashback() = (%d, %v), want (0, ErrNegativeKopecks)", amount, err)
	}
	assertReservationState(t, fixture, nil, nil)
}

func TestReserveCashbackOrderConflictDoesNotWriteIntegration(t *testing.T) {
	for _, name := range []string{"different_user", "different_month"} {
		t.Run(name, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			ctx := storageTestContext(t)
			period := reservationTestPeriod()
			if amount, err := fixture.ReserveCashback(ctx, "order-1", "user-1", period, 5_000); err != nil || amount != 5_000 {
				t.Fatalf("initial ReserveCashback() = (%d, %v), want (5000, nil)", amount, err)
			}
			userID := "user-1"
			if name == "different_user" {
				userID = "user-2"
			} else {
				period = period.AddDate(0, 1, 0)
			}
			amount, err := fixture.ReserveCashback(ctx, "order-1", userID, period, 5_000)
			if amount != 0 || !errors.Is(err, storagepkg.ErrOrderConflict) {
				t.Fatalf("conflicting ReserveCashback() = (%d, %v), want (0, ErrOrderConflict)", amount, err)
			}
			// The temporary new user/month row must also be rolled back.
			assertReservationState(t, fixture,
				[]monthlyLimitRow{{"user-1", "2026-11-01", 100_000, 5_000, 0}},
				[]reservationRow{{"order-1", "user-1", "2026-11-01", 5_000, "reserved"}},
			)
		})
	}
}

func TestReserveCashbackRollsBackOnInsertErrorIntegration(t *testing.T) {
	for _, name := range []string{"existing_month", "new_month"} {
		t.Run(name, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			ctx := storageTestContext(t)
			var wantLimits []monthlyLimitRow
			if name == "existing_month" {
				wantLimits = []monthlyLimitRow{{"user-1", "2026-11-01", 100_000, 0, 94_000}}
				seedMonthlyLimit(t, fixture, wantLimits[0])
			}
			// Inject a failure after the monthly counter has been updated.
			// Only this test's isolated reservation table is changed.
			if _, err := fixture.pool.Exec(ctx, `
				ALTER TABLE cashback_reservations ADD CONSTRAINT reject_test_order
				CHECK (order_id <> 'order-1')
			`); err != nil {
				t.Fatal(err)
			}
			amount, err := fixture.ReserveCashback(ctx, "order-1", "user-1", reservationTestPeriod(), 5_000)
			var pgErr *pgconn.PgError
			if amount != 0 || !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("ReserveCashback() = (%d, %v), want (0, check constraint violation)", amount, err)
			}
			assertReservationState(t, fixture, wantLimits, nil)

			if _, err := fixture.pool.Exec(ctx, `
				ALTER TABLE cashback_reservations DROP CONSTRAINT reject_test_order
			`); err != nil {
				t.Fatal(err)
			}
			// Retry through the same single-connection storage. This also
			// proves rollback released its transaction and connection.
			amount, err = fixture.ReserveCashback(ctx, "order-1", "user-1", reservationTestPeriod(), 5_000)
			if err != nil || amount != 5_000 {
				t.Fatalf("retry ReserveCashback() = (%d, %v), want (5000, nil)", amount, err)
			}
			credited := int64(0)
			if name == "existing_month" {
				credited = 94_000
			}
			assertReservationState(t, fixture,
				[]monthlyLimitRow{{"user-1", "2026-11-01", 100_000, 5_000, credited}},
				[]reservationRow{{"order-1", "user-1", "2026-11-01", 5_000, "reserved"}},
			)
		})
	}
}

func TestReserveCashbackCommitErrorIsNotSuccessIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	// A deferred foreign key allows both writes, then fails at Commit.
	if _, err := fixture.pool.Exec(ctx, `
		CREATE TABLE allowed_test_orders (order_id TEXT PRIMARY KEY);
		ALTER TABLE cashback_reservations ADD CONSTRAINT allow_test_order
		FOREIGN KEY (order_id) REFERENCES allowed_test_orders(order_id)
		DEFERRABLE INITIALLY DEFERRED;
	`); err != nil {
		t.Fatal(err)
	}
	amount, err := fixture.ReserveCashback(ctx, "order-1", "user-1", reservationTestPeriod(), 5_000)
	var pgErr *pgconn.PgError
	if amount != 0 || !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("ReserveCashback() = (%d, %v), want (0, deferred foreign key violation)", amount, err)
	}
	assertReservationState(t, fixture, nil, nil)

	if _, err := fixture.pool.Exec(ctx, `INSERT INTO allowed_test_orders VALUES ('order-1')`); err != nil {
		t.Fatal(err)
	}
	amount, err = fixture.ReserveCashback(ctx, "order-1", "user-1", reservationTestPeriod(), 5_000)
	if err != nil || amount != 5_000 {
		t.Fatalf("retry ReserveCashback() = (%d, %v), want (5000, nil)", amount, err)
	}
	assertReservationState(t, fixture,
		[]monthlyLimitRow{{"user-1", "2026-11-01", 100_000, 5_000, 0}},
		[]reservationRow{{"order-1", "user-1", "2026-11-01", 5_000, "reserved"}},
	)
}
