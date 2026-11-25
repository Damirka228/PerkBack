package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
)

func TestReserveCashbackExistingReservationSkipsCalculationAndCreation(t *testing.T) {
	for _, status := range []string{"reserved", "credited"} {
		for _, amount := range []int64{0, 2_000} {
			t.Run(fmt.Sprintf("%s/amount_%d", status, amount), func(t *testing.T) {
				ctx := t.Context()
				input := reservationInput()
				// Deliberately use a past month: a retry must use the saved
				// result, not reserve against the current month's limit.
				current := time.Now().UTC()
				previousMonth := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
				repository := &fakeCashbackReserver{
					existing: &domain.CashbackReservation{
						UserID: input.UserID, PeriodStart: previousMonth,
						AmountKop: amount, Status: status,
					},
					err: errors.New("must not create another reservation"),
				}
				// Even unavailable rules must not prevent reading a saved result.
				provider := &optionsProviderTest{err: errors.New("rules unavailable")}
				service := usecase.NewService(provider, repository, nil)

				got, err := service.ReserveCashback(ctx, "order-1", input)
				if err != nil || got != amount {
					t.Fatalf("ReserveCashback() = (%d, %v), want (%d, nil)", got, err, amount)
				}
				assertReservationLookup(t, repository, ctx, "order-1")
				if provider.calls != 0 || len(repository.calls) != 0 {
					t.Errorf("replay recalculated or created a reservation: calculations=%d, reservations=%d",
						provider.calls, len(repository.calls))
				}
				if repository.existing.PeriodStart != previousMonth || repository.existing.Status != status ||
					repository.existing.AmountKop != amount {
					t.Error("replay changed the saved reservation")
				}
			})
		}
	}
}

func TestReserveCashbackCanceledReservationRejectsReplay(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		ownerID string
		wantErr error
	}{
		{name: "zero_amount", ownerID: "user-1", wantErr: domain.ErrReservationCanceled},
		{name: "positive_amount", amount: 2_000, ownerID: "user-1", wantErr: domain.ErrReservationCanceled},
		{name: "another_user", amount: 2_000, ownerID: "another-user", wantErr: domain.ErrNotUserID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			input := reservationInput()
			existing := domain.CashbackReservation{
				UserID:      tt.ownerID,
				PeriodStart: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC),
				AmountKop:   tt.amount,
				Status:      "canceled",
			}
			repository := &fakeCashbackReserver{
				existing: &existing,
				err:      errors.New("must not create another reservation"),
			}
			provider := &optionsProviderTest{err: errors.New("must not recalculate canceled order")}
			service := usecase.NewService(provider, repository, nil)
			before := *repository.existing

			got, err := service.ReserveCashback(ctx, "order-1", input)
			if got != 0 || !errors.Is(err, tt.wantErr) {
				t.Errorf("ReserveCashback() = (%d, %v), want (0, %v)", got, err, tt.wantErr)
			}
			assertReservationLookup(t, repository, ctx, "order-1")
			if provider.calls != 0 || len(repository.calls) != 0 {
				t.Errorf("canceled replay calculated or created a reservation: calculations=%d, reservations=%d",
					provider.calls, len(repository.calls))
			}
			if *repository.existing != before {
				t.Error("replay changed the saved reservation")
			}
		})
	}
}

func TestReserveCashbackRepeatAfterRulesChangeReturnsSavedAmount(t *testing.T) {
	ctx := t.Context()
	input := reservationInput()
	provider := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 500}}}
	repository := &fakeCashbackReserver{amount: 2_000}
	service := usecase.NewService(provider, repository, nil)

	first, err := service.ReserveCashback(ctx, "order-1", input)
	if err != nil || first != 2_000 {
		t.Fatalf("first ReserveCashback() = (%d, %v), want (2000, nil)", first, err)
	}
	if len(repository.calls) != 1 || repository.calls[0].requestedKopecks != 5_000 {
		t.Fatalf("first request must reserve calculated cashback 5000; calls = %+v", repository.calls)
	}
	// Simulate the committed result now being visible to the lookup.
	repository.existing = &domain.CashbackReservation{
		UserID: input.UserID, PeriodStart: repository.calls[0].periodStart,
		AmountKop: first, Status: "reserved",
	}
	provider.variants = []usecase.BestVariant{{RateBPS: 1_000}}
	repository.amount = 10_000

	second, err := service.ReserveCashback(ctx, "order-1", input)
	if err != nil || second != first {
		t.Fatalf("replayed ReserveCashback() = (%d, %v), want (%d, nil)", second, err, first)
	}
	if provider.calls != 1 || len(repository.calls) != 1 || len(repository.lookupCalls) != 2 {
		t.Errorf("repeat must only look up the saved reservation; calculations=%d, reservations=%d, lookups=%d",
			provider.calls, len(repository.calls), len(repository.lookupCalls))
	}
}

func TestReserveCashbackExistingOrderRejectsAnotherUser(t *testing.T) {
	ctx := t.Context()
	provider := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 500}}}
	repository := &fakeCashbackReserver{
		existing: &domain.CashbackReservation{
			UserID: "another-user", PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			AmountKop: 2_000, Status: "reserved",
		},
	}
	service := usecase.NewService(provider, repository, nil)

	amount, err := service.ReserveCashback(ctx, "order-1", reservationInput())
	if amount != 0 || !errors.Is(err, domain.ErrNotUserID) {
		t.Fatalf("ReserveCashback() = (%d, %v), want (0, ErrNotUserID)", amount, err)
	}
	assertReservationLookup(t, repository, ctx, "order-1")
	if provider.calls != 0 || len(repository.calls) != 0 {
		t.Error("a foreign order must not trigger calculation or reservation creation")
	}
}

func TestReserveCashbackLookupErrorStopsOperation(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "database_error", err: errors.New("database unavailable")},
		{name: "canceled", err: context.Canceled},
		{name: "deadline", err: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			provider := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 500}}}
			repository := &fakeCashbackReserver{
				lookupErr: fmt.Errorf("lookup failed: %w", tt.err),
				amount:    5_000,
			}
			service := usecase.NewService(provider, repository, nil)

			amount, err := service.ReserveCashback(ctx, "order-1", reservationInput())
			if amount != 0 || !errors.Is(err, tt.err) {
				t.Fatalf("ReserveCashback() = (%d, %v), want (0, wrapped %v)", amount, err, tt.err)
			}
			assertReservationLookup(t, repository, ctx, "order-1")
			if provider.calls != 0 || len(repository.calls) != 0 {
				t.Error("lookup failure must not be treated as a new order")
			}
		})
	}
}

func TestReserveCashbackWrappedNotFoundContinuesCreation(t *testing.T) {
	ctx := t.Context()
	provider := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 500}}}
	repository := &fakeCashbackReserver{
		lookupErr: fmt.Errorf("lookup order: %w", domain.ErrReservationNotFound),
		amount:    2_000,
	}
	service := usecase.NewService(provider, repository, nil)

	amount, err := service.ReserveCashback(ctx, "order-1", reservationInput())
	if err != nil || amount != 2_000 {
		t.Fatalf("ReserveCashback() = (%d, %v), want (2000, nil)", amount, err)
	}
	assertReservationLookup(t, repository, ctx, "order-1")
	if provider.calls != 1 || len(repository.calls) != 1 {
		t.Fatal("wrapped ErrReservationNotFound must continue to calculation and creation")
	}
}
