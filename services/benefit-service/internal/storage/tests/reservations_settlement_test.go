package storage_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	settlementOrder  = "order-1"
	settlementUser   = "user-1"
	settlementAmount = int64(5_000)
)

func seedSettlement(t *testing.T, fixture *storageFixture) (monthlyLimitRow, []reservationRow) {
	t.Helper()
	period := reservationTestPeriod()
	limit := monthlyLimitRow{UserID: settlementUser, Period: period.Format(time.DateOnly), Limit: 100_000, Credited: 94_000}
	seedMonthlyLimit(t, fixture, limit)
	orders := []reservationRow{
		{OrderID: settlementOrder, UserID: limit.UserID, Period: limit.Period, Amount: settlementAmount, Status: "reserved"},
		{OrderID: "order-2", UserID: limit.UserID, Period: limit.Period, Amount: 1_000, Status: "reserved"},
	}
	ctx := storageTestContext(t)
	for _, order := range orders {
		got, err := fixture.ReserveCashback(ctx, order.OrderID, order.UserID, period, order.Amount)
		if err != nil || got != order.Amount {
			t.Fatalf("seed ReserveCashback(%q) = (%d, %v), want (%d, nil)", order.OrderID, got, err, order.Amount)
		}
		limit.Reserved += order.Amount
	}
	return limit, orders
}

func TestCashbackSettlementIsIdempotentIntegration(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		fixture := newIntegrationStorage(t)
		limit, orders := seedSettlement(t, fixture)
		ctx := storageTestContext(t)
		limit.Reserved = 1_000
		limit.Credited = 99_000
		orders[0].Status = "credited"

		for attempt := 1; attempt <= 2; attempt++ {
			amount, err := fixture.ConfirmCashback(ctx, settlementOrder, settlementUser)
			if err != nil || amount != settlementAmount {
				t.Fatalf("ConfirmCashback attempt %d = (%d, %v), want (%d, nil)", attempt, amount, err, settlementAmount)
			}
			assertReservationState(t, fixture, []monthlyLimitRow{limit}, orders)
		}
		if err := fixture.CancelCashback(ctx, settlementOrder, settlementUser); !errors.Is(err, storage.ErrCanceledOrder) {
			t.Fatalf("cancel credited reservation: %v, want ErrCanceledOrder", err)
		}
		assertReservationState(t, fixture, []monthlyLimitRow{limit}, orders)
	})

	t.Run("cancel", func(t *testing.T) {
		fixture := newIntegrationStorage(t)
		limit, orders := seedSettlement(t, fixture)
		ctx := storageTestContext(t)
		limit.Reserved = 1_000
		orders[0].Status = "canceled"

		for attempt := 1; attempt <= 2; attempt++ {
			if err := fixture.CancelCashback(ctx, settlementOrder, settlementUser); err != nil {
				t.Fatalf("CancelCashback attempt %d: %v", attempt, err)
			}
			assertReservationState(t, fixture, []monthlyLimitRow{limit}, orders)
		}
		if amount, err := fixture.ConfirmCashback(ctx, settlementOrder, settlementUser); amount != 0 || !errors.Is(err, storage.ErrStatusCancel) {
			t.Fatalf("confirm canceled reservation = (%d, %v), want (0, ErrStatusCancel)", amount, err)
		}
		assertReservationState(t, fixture, []monthlyLimitRow{limit}, orders)
	})
}

func TestCashbackSettlementRejectsMissingAndForeignOrdersIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	limit, orders := seedSettlement(t, fixture)
	for _, tt := range []struct {
		name, orderID, userID string
		want                  error
	}{
		{"missing", "missing-order", settlementUser, domain.ErrReservationNotFound},
		{"foreign", settlementOrder, "other-user", domain.ErrNotUserID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := storageTestContext(t)
			if amount, err := fixture.ConfirmCashback(ctx, tt.orderID, tt.userID); amount != 0 || !errors.Is(err, tt.want) {
				t.Errorf("ConfirmCashback = (%d, %v), want (0, %v)", amount, err, tt.want)
			}
			if err := fixture.CancelCashback(ctx, tt.orderID, tt.userID); !errors.Is(err, tt.want) {
				t.Errorf("CancelCashback = %v, want %v", err, tt.want)
			}
			assertReservationState(t, fixture, []monthlyLimitRow{limit}, orders)
		})
	}
}

func TestCashbackSettlementRollsBackOnStatusUpdateErrorIntegration(t *testing.T) {
	for _, operation := range []string{"confirm", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			limit, orders := seedSettlement(t, fixture)
			ctx := storageTestContext(t)
			// The monthly update can succeed, but changing the order status fails.
			if _, err := fixture.pool.Exec(ctx, `
				ALTER TABLE cashback_reservations ADD CONSTRAINT reject_settlement
				CHECK (status = 'reserved')
			`); err != nil {
				t.Fatalf("install test constraint: %v", err)
			}

			var amount int64
			var err error
			if operation == "confirm" {
				amount, err = fixture.ConfirmCashback(ctx, settlementOrder, settlementUser)
			} else {
				err = fixture.CancelCashback(ctx, settlementOrder, settlementUser)
			}
			var pgErr *pgconn.PgError
			if amount != 0 || !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "reject_settlement" {
				t.Fatalf("%s = (%d, %v), want zero amount and reject_settlement violation", operation, amount, err)
			}
			assertReservationState(t, fixture, []monthlyLimitRow{limit}, orders)
		})
	}
}

func TestCashbackConfirmAndCancelCompeteForSameOrderIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	limit, orders := seedSettlement(t, fixture)
	confirmer, confirmName := newConcurrentClient(t, fixture)
	canceler, cancelName := newConcurrentClient(t, fixture)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	blocker, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		cleanupCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := blocker.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("release test lock: %v", err)
		}
		workers.Wait()
	})
	if _, err := blocker.Exec(ctx, `
		SELECT 1 FROM cashback_reservations WHERE order_id = $1 FOR UPDATE
	`, settlementOrder); err != nil {
		t.Fatalf("hold reservation lock: %v", err)
	}

	type confirmation struct {
		amount int64
		err    error
	}
	confirmed := make(chan confirmation, 1)
	canceled := make(chan error, 1)
	workers.Add(2)
	go func() {
		defer workers.Done()
		amount, err := confirmer.ConfirmCashback(ctx, settlementOrder, settlementUser)
		confirmed <- confirmation{amount, err}
	}()
	go func() {
		defer workers.Done()
		canceled <- canceler.CancelCashback(ctx, settlementOrder, settlementUser)
	}()

	// Both operations must reach the database before either can change status.
	waitForReservationLockWaiters(t, fixture, ctx, []string{confirmName, cancelName})
	if err := blocker.Commit(ctx); err != nil {
		t.Fatalf("release reservation lock: %v", err)
	}

	var confirmResult confirmation
	select {
	case confirmResult = <-confirmed:
	case <-ctx.Done():
		t.Fatalf("confirmation did not finish: %v", ctx.Err())
	}
	var cancelErr error
	select {
	case cancelErr = <-canceled:
	case <-ctx.Done():
		t.Fatalf("cancellation did not finish: %v", ctx.Err())
	}

	limit.Reserved = 1_000
	switch {
	case confirmResult.err == nil && confirmResult.amount == settlementAmount && errors.Is(cancelErr, storage.ErrCanceledOrder):
		limit.Credited = 99_000
		orders[0].Status = "credited"
	case cancelErr == nil && confirmResult.amount == 0 && errors.Is(confirmResult.err, storage.ErrStatusCancel):
		orders[0].Status = "canceled"
	default:
		t.Fatalf("expected exactly one valid transition; confirm = (%d, %v), cancel = %v",
			confirmResult.amount, confirmResult.err, cancelErr)
	}
	assertReservationState(t, fixture, []monthlyLimitRow{limit}, orders)
}
