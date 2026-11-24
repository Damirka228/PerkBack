package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
)

type reservationCall struct {
	ctx              context.Context
	orderID          string
	userID           string
	periodStart      time.Time
	requestedKopecks int64
}

type fakeCashbackReserver struct {
	calls       []reservationCall
	amount      int64
	err         error
	lookupCalls []reservationLookupCall
	existing    *domain.CashbackReservation
	lookupErr   error
}

type reservationLookupCall struct {
	ctx     context.Context
	orderID string
}

func (f *fakeCashbackReserver) GetReservByOrderID(ctx context.Context, orderID string) (domain.CashbackReservation, error) {
	f.lookupCalls = append(f.lookupCalls, reservationLookupCall{ctx, orderID})
	if f.lookupErr != nil {
		return domain.CashbackReservation{}, f.lookupErr
	}
	if f.existing == nil {
		return domain.CashbackReservation{}, domain.ErrReservationNotFound
	}
	return *f.existing, nil
}

func (f *fakeCashbackReserver) ReserveCashback(
	ctx context.Context,
	orderID, userID string,
	periodStart time.Time,
	requestedKopecks int64,
) (int64, error) {
	f.calls = append(f.calls, reservationCall{ctx, orderID, userID, periodStart, requestedKopecks})
	return f.amount, f.err
}

func reservationInput() usecase.CalculateInput {
	return usecase.CalculateInput{
		UserID:            "user-1",
		ShopID:            "shop-42",
		CategoryID:        "groceries",
		PurchaseAmountKop: 100_000,
	}
}

func TestReserveCashbackSuccess(t *testing.T) {
	tests := []struct {
		name          string
		purchase      int64
		variants      []usecase.BestVariant
		wantRequested int64
		reserved      int64
	}{
		{
			name: "full_cashback", purchase: 100_000,
			variants:      []usecase.BestVariant{{RateBPS: 500}},
			wantRequested: 5_000, reserved: 5_000,
		},
		{
			name: "returns_actual_reservation_not_calculated_cashback", purchase: 100_000,
			variants:      []usecase.BestVariant{{RateBPS: 500}},
			wantRequested: 5_000, reserved: 2_000,
		},
		{
			name: "exhausted_limit_returns_zero", purchase: 100_000,
			variants:      []usecase.BestVariant{{RateBPS: 500}},
			wantRequested: 5_000, reserved: 0,
		},
		{
			name: "reserves_only_cashback_of_best_total_benefit_option", purchase: 100_000,
			variants: []usecase.BestVariant{
				{RateBPS: 500},
				{DiscountKop: 7_000, RateBPS: 300},
			},
			// 3% of 93000, not the 9790 total benefit or the other option's 5000.
			wantRequested: 2_790, reserved: 2_000,
		},
		{
			name: "zero_cashback_still_calls_repository", purchase: 100_000,
			variants: []usecase.BestVariant{{}},
		},
		{
			name: "full_discount_still_reserves_zero", purchase: 100_000,
			variants: []usecase.BestVariant{{DiscountKop: 100_000, RateBPS: 500}},
		},
		{
			name:     "zero_purchase_still_reserves_zero",
			variants: []usecase.BestVariant{{RateBPS: 500}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := reservationInput()
			input.PurchaseAmountKop = tt.purchase
			provider := &optionsProviderTest{variants: tt.variants}
			repository := &fakeCashbackReserver{amount: tt.reserved}
			service := usecase.NewService(provider, repository)
			ctx := t.Context()

			before := time.Now().UTC()
			amount, err := service.ReserveCashback(ctx, "order-1", input)
			after := time.Now().UTC()
			if err != nil || amount != tt.reserved {
				t.Fatalf("ReserveCashback() = (%d, %v), want (%d, nil)", amount, err, tt.reserved)
			}
			if provider.calls != 1 || provider.input != input || provider.ctx != ctx {
				t.Fatalf("GetOptions must receive the original context and input exactly once; calls=%d, input=%+v",
					provider.calls, provider.input)
			}
			assertReservationLookup(t, repository, ctx, "order-1")
			if len(repository.calls) != 1 {
				t.Fatalf("repository calls = %d, want 1", len(repository.calls))
			}
			call := repository.calls[0]
			if call.ctx != ctx {
				t.Error("repository did not receive the original context")
			}
			if call.orderID != "order-1" || call.userID != input.UserID || call.requestedKopecks != tt.wantRequested {
				t.Errorf("repository arguments = (order=%q, user=%q, requested=%d), want (order-1, %s, %d)",
					call.orderID, call.userID, call.requestedKopecks, input.UserID, tt.wantRequested)
			}
			assertCurrentUTCPeriod(t, call.periodStart, before, after)
		})
	}
}

func TestReserveCashbackValidationDoesNotCallDependencies(t *testing.T) {
	tests := []struct {
		name    string
		orderID string
		userID  string
		wantErr error
	}{
		{name: "empty_order", userID: "user-1", wantErr: usecase.ErrOrderIDEmpty},
		{name: "whitespace_order", orderID: " \t\n", userID: "user-1", wantErr: usecase.ErrOrderIDEmpty},
		{name: "empty_user", orderID: "order-1", wantErr: usecase.ErrUserIDEmpty},
		{name: "whitespace_user", orderID: "order-1", userID: " \t\n", wantErr: usecase.ErrUserIDEmpty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := reservationInput()
			input.UserID = tt.userID
			provider := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 500}}}
			repository := &fakeCashbackReserver{amount: 5_000}
			service := usecase.NewService(provider, repository)

			amount, err := service.ReserveCashback(t.Context(), tt.orderID, input)
			if amount != 0 || !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReserveCashback() = (%d, %v), want (0, %v)", amount, err, tt.wantErr)
			}
			if provider.calls != 0 || len(repository.lookupCalls) != 0 || len(repository.calls) != 0 {
				t.Errorf("invalid input called dependencies: provider=%d, lookups=%d, reservations=%d",
					provider.calls, len(repository.lookupCalls), len(repository.calls))
			}
		})
	}
}

func TestReserveCashbackCalculationErrorDoesNotCreateReservation(t *testing.T) {
	providerErr := errors.New("options unavailable")
	tests := []struct {
		name          string
		purchase      int64
		variants      []usecase.BestVariant
		providerErr   error
		wantErr       error
		providerCalls int
	}{
		{name: "negative_purchase", purchase: -1, wantErr: domain.ErrInvalidAmount},
		{
			name: "provider_error", purchase: 100_000,
			providerErr: providerErr, wantErr: providerErr, providerCalls: 1,
		},
		{
			name: "provider_canceled", purchase: 100_000,
			providerErr: context.Canceled, wantErr: context.Canceled, providerCalls: 1,
		},
		{name: "no_options", purchase: 100_000, providerCalls: 1},
		{
			name: "invalid_rate", purchase: 100_000,
			variants: []usecase.BestVariant{{RateBPS: 10_001}},
			wantErr:  domain.ErrInvalidRate, providerCalls: 1,
		},
		{
			name: "invalid_discount", purchase: 100_000,
			variants: []usecase.BestVariant{{DiscountKop: 100_001}},
			wantErr:  domain.ErrInvalidDiscount, providerCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := reservationInput()
			input.PurchaseAmountKop = tt.purchase
			provider := &optionsProviderTest{variants: tt.variants, err: tt.providerErr}
			repository := &fakeCashbackReserver{amount: 5_000}
			service := usecase.NewService(provider, repository)

			ctx := t.Context()
			amount, err := service.ReserveCashback(ctx, "order-1", input)
			if amount != 0 || err == nil {
				t.Fatalf("ReserveCashback() = (%d, %v), want zero and a calculation error", amount, err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want wrapped %v", err, tt.wantErr)
			}
			if provider.calls != tt.providerCalls {
				t.Errorf("GetOptions calls = %d, want %d", provider.calls, tt.providerCalls)
			}
			assertReservationLookup(t, repository, ctx, "order-1")
			if len(repository.calls) != 0 {
				t.Errorf("repository calls = %d, want 0 after calculation failure", len(repository.calls))
			}
		})
	}
}

func TestReserveCashbackPreservesRepositoryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "repository_failure", err: errors.New("reservation storage unavailable")},
		{name: "cancellation", err: context.Canceled},
		{name: "deadline", err: context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 500}}}
			// A repository result must not be reported as success alongside an error.
			repository := &fakeCashbackReserver{amount: 2_000, err: tt.err}
			service := usecase.NewService(provider, repository)
			ctx := t.Context()

			amount, err := service.ReserveCashback(ctx, "order-1", reservationInput())
			if amount != 0 || !errors.Is(err, tt.err) {
				t.Fatalf("ReserveCashback() = (%d, %v), want (0, wrapped %v)", amount, err, tt.err)
			}
			if provider.calls != 1 || len(repository.calls) != 1 {
				t.Fatalf("dependency calls = (provider=%d, repository=%d), want (1, 1)",
					provider.calls, len(repository.calls))
			}
			assertReservationLookup(t, repository, ctx, "order-1")
			if repository.calls[0].ctx != ctx || repository.calls[0].requestedKopecks != 5_000 {
				t.Error("repository did not receive the original context and calculated cashback")
			}
		})
	}
}

func TestCalculateDoesNotReserveCashback(t *testing.T) {
	provider := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 500}}}
	repository := &fakeCashbackReserver{amount: 2_000}
	service := usecase.NewService(provider, repository)

	got, err := service.Calculate(t.Context(), reservationInput())
	want := domain.OptionResult{PayableKop: 100_000, CashbackKop: 5_000}
	if err != nil || got != want {
		t.Fatalf("Calculate() = (%+v, %v), want (%+v, nil)", got, err, want)
	}
	if len(repository.calls) != 0 || len(repository.lookupCalls) != 0 {
		t.Errorf("Calculate accessed reservation repository: lookups=%d, reservations=%d, want none",
			len(repository.lookupCalls), len(repository.calls))
	}
}

func assertReservationLookup(t *testing.T, repository *fakeCashbackReserver, ctx context.Context, orderID string) {
	t.Helper()
	if len(repository.lookupCalls) != 1 {
		t.Fatalf("reservation lookups = %d, want 1", len(repository.lookupCalls))
	}
	call := repository.lookupCalls[0]
	if call.ctx != ctx || call.orderID != orderID {
		t.Errorf("lookup must receive the original context and order ID %q; got order ID %q",
			orderID, call.orderID)
	}
}

func assertCurrentUTCPeriod(t *testing.T, period, before, after time.Time) {
	t.Helper()
	monthStart := func(now time.Time) time.Time {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	// Accept either month if the call happens across midnight at a month
	// boundary. No clock injection or production changes are needed.
	first, last := monthStart(before), monthStart(after)
	if period.Location() != time.UTC || (!period.Equal(first) && !period.Equal(last)) {
		t.Errorf("repository period = %v (%s), want UTC month start %v or %v",
			period, period.Location(), first, last)
	}
}
