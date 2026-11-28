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

func TestCalculatePersonalization(t *testing.T) {
	base := domain.OptionResult{
		PayableKop:            100_000,
		CashbackKop:           6_000,
		PersonalizationStatus: domain.PersonalizationNotApplied,
	}
	fallback := base
	fallback.PersonalizationStatus = domain.PersonalizationUnavailable

	for _, tt := range []struct {
		name       string
		categories []string
		profileErr error
		want       domain.OptionResult
	}{
		{
			// At 300 BPS the discounted option loses; at 500 BPS it wins.
			name:       "selected_category_changes_winner",
			categories: []string{"electronics", "groceries"},
			want: domain.OptionResult{
				DiscountKop:           2_000,
				PayableKop:            98_000,
				CashbackKop:           4_900,
				PersonalizationStatus: domain.PersonalizationApplied,
			},
		},
		{name: "other_category_selected", categories: []string{"electronics"}, want: base},
		{name: "missing_profile", profileErr: fmt.Errorf("get profile: %w", domain.ErrProfileNotFound), want: base},
		{name: "user_service_unavailable", profileErr: fmt.Errorf("get profile: %w", domain.ErrUserServiceUnavailable), want: fallback},
	} {
		t.Run(tt.name, func(t *testing.T) {
			profiles := &userProviderTest{categories: tt.categories, err: tt.profileErr}
			options := &optionsProviderTest{variants: []usecase.BestVariant{
				{RateBPS: 600},
				{DiscountKop: 2_000, RateBPS: 300},
			}}
			service := usecase.NewService(options, nil, nil, profiles, 500)

			got, err := service.Calculate(t.Context(), reservationInput())
			if err != nil || got != tt.want {
				t.Fatalf("Calculate() = (%+v, %v), want (%+v, nil)", got, err, tt.want)
			}
			if profiles.calls != 1 {
				t.Errorf("GetProfile calls = %d, want 1", profiles.calls)
			}
		})
	}
}

type userProviderFunc func(context.Context, string) (domain.UserProfile, error)

func (f userProviderFunc) GetProfile(ctx context.Context, userID string) (domain.UserProfile, error) {
	return f(ctx, userID)
}

func TestCalculateRequestContextTakesPrecedenceOverFallback(t *testing.T) {
	t.Run("canceled_during_lookup", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		profiles := userProviderFunc(func(context.Context, string) (domain.UserProfile, error) {
			cancel()
			return domain.UserProfile{}, domain.ErrUserServiceUnavailable
		})
		assertCalculateStopsBeforeOptions(t, ctx, profiles, context.Canceled)
	})

	t.Run("parent_deadline_exceeded", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
		defer cancel()
		profiles := &userProviderTest{err: domain.ErrUserServiceUnavailable}
		assertCalculateStopsBeforeOptions(t, ctx, profiles, context.DeadlineExceeded)
	})
}

func assertCalculateStopsBeforeOptions(t *testing.T, ctx context.Context, profiles usecase.UserProvider, wantErr error) {
	t.Helper()
	options := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 100}}}
	service := usecase.NewService(options, nil, nil, profiles, 500)
	got, err := service.Calculate(ctx, reservationInput())
	if got != (domain.OptionResult{}) || !errors.Is(err, wantErr) {
		t.Fatalf("Calculate() = (%+v, %v), want empty result and %v", got, err, wantErr)
	}
	if options.calls != 0 {
		t.Errorf("GetOptions calls = %d, want 0", options.calls)
	}
}

func TestReserveCashbackDoesNotPersistFallback(t *testing.T) {
	ctx := t.Context()
	profiles := &userProviderTest{err: domain.ErrUserServiceUnavailable}
	options := &optionsProviderTest{variants: []usecase.BestVariant{{RateBPS: 100}}}
	repo := &fakeCashbackReserver{amount: 1_000}
	service := usecase.NewService(options, repo, nil, profiles, 500)

	got, err := service.ReserveCashback(ctx, "order-1", reservationInput())
	if got != 0 || !errors.Is(err, domain.ErrUserServiceUnavailable) {
		t.Fatalf("ReserveCashback() = (%d, %v), want (0, %v)", got, err, domain.ErrUserServiceUnavailable)
	}
	assertReservationLookup(t, repo, ctx, "order-1")
	if len(repo.calls) != 0 {
		t.Errorf("reservation writes = %d, want 0", len(repo.calls))
	}
}
