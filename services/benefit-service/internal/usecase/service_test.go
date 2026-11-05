package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"github.com/go-playground/assert"
)

type rateProviderTest struct {
	rateBPS int64
	err     error
	calls   int
	ctx     context.Context
	input   usecase.CalculateInput
}

func (p *rateProviderTest) GetRateBPS(ctx context.Context, input usecase.CalculateInput) (int64, error) {
	p.calls++
	p.ctx = ctx
	p.input = input
	return p.rateBPS, p.err
}

func TestCalculateSuccess(t *testing.T) {
	provider := &rateProviderTest{rateBPS: 300}
	service := usecase.NewService(provider)

	ctx := t.Context()
	input := usecase.CalculateInput{
		UserID:            "user-1",
		MerchantID:        "shop-42",
		CategoryID:        "groceries",
		PurchaseAmountKop: 100_000,
	}
	res, err := service.Calculate(ctx, input)
	if err != nil {
		t.Fatalf("err test: %s", err)
	}
	assert.Equal(t, res, usecase.CalculateResult{
		DiscountKop: 0,
		PayableKop:  100_000,
		CashbackKop: 3_000,
	})
	if provider.calls != 1 {
		t.Fatalf("GetRateBPS calls = %d, want 1", provider.calls)
	}
	if provider.ctx != ctx {
		t.Error("GetRateBPS did not receive the original context")
	}
	if provider.input != input {
		t.Errorf("GetRateBPS input = %+v, want %+v", provider.input, input)
	}
}

func TestCalculateErrors(t *testing.T) {
	tests := []struct {
		name        string
		amount      int64
		rateBPS     int64
		providerErr error
		wantErr     error
		wantCalls   int
	}{
		{
			name:    "negative_amount",
			amount:  -1,
			wantErr: domain.ErrInvalidAmount,
		},
		{
			name:        "provider_deadline_exceeded",
			amount:      100_000,
			providerErr: context.DeadlineExceeded,
			wantErr:     context.DeadlineExceeded,
			wantCalls:   1,
		},
		{
			name:      "invalid_provider_rate",
			amount:    100_000,
			rateBPS:   10_001,
			wantErr:   domain.ErrInvalidRate,
			wantCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &rateProviderTest{rateBPS: tc.rateBPS, err: tc.providerErr}
			service := usecase.NewService(provider)

			res, err := service.Calculate(t.Context(), usecase.CalculateInput{
				PurchaseAmountKop: tc.amount,
			})

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Calculate error = %v, want %v", err, tc.wantErr)
			}
			if res != (usecase.CalculateResult{}) {
				t.Errorf("Calculate result = %+v, want an empty result", res)
			}
			if provider.calls != tc.wantCalls {
				t.Errorf("GetRateBPS calls = %d, want %d", provider.calls, tc.wantCalls)
			}
		})
	}
}
