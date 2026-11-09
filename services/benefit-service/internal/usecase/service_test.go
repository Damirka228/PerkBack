package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"github.com/go-playground/assert"
)

type optionsProviderTest struct {
	variants []usecase.BestVariant
	err      error
	calls    int
	ctx      context.Context
	input    usecase.CalculateInput
}

func (p *optionsProviderTest) GetRateBPS(ctx context.Context, input usecase.CalculateInput) ([]usecase.BestVariant, error) {
	p.calls++
	p.ctx = ctx
	p.input = input
	return p.variants, p.err
}

func TestCalculateSuccess(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		variants []usecase.BestVariant
		want     domain.OptionResult
	}{
		{
			name:   "single_option",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{RateBPS: 300},
			},
			want: domain.OptionResult{
				PayableKop:  100_000,
				CashbackKop: 3_000,
			},
		},
		{
			name:   "lower_rate_with_discount_gives_best_total_benefit",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{RateBPS: 500},
				{DiscountKop: 7_000, RateBPS: 300},
			},
			want: domain.OptionResult{
				DiscountKop: 7_000,
				PayableKop:  93_000,
				CashbackKop: 2_790,
			},
		},
		{
			name:   "later_worse_options_do_not_overwrite_best",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{DiscountKop: 7_000, RateBPS: 300},
				{RateBPS: 500},
				{RateBPS: 100},
			},
			want: domain.OptionResult{
				DiscountKop: 7_000,
				PayableKop:  93_000,
				CashbackKop: 2_790,
			},
		},
		{
			name:   "equal_benefit_keeps_first_cashback_option",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{RateBPS: 500},
				{DiscountKop: 5_000},
			},
			want: domain.OptionResult{
				PayableKop:  100_000,
				CashbackKop: 5_000,
			},
		},
		{
			name:   "equal_benefit_keeps_first_discount_option",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{DiscountKop: 5_000},
				{RateBPS: 500},
			},
			want: domain.OptionResult{
				DiscountKop: 5_000,
				PayableKop:  95_000,
			},
		},
		{
			name:   "zero_benefit_preserves_payable_amount",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{},
			},
			want: domain.OptionResult{
				PayableKop: 100_000,
			},
		},
		{
			name:   "full_discount",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{RateBPS: 500},
				{DiscountKop: 100_000, RateBPS: 300},
			},
			want: domain.OptionResult{
				DiscountKop: 100_000,
			},
		},
		{
			name: "zero_purchase",
			variants: []usecase.BestVariant{
				{RateBPS: 100},
				{RateBPS: 300},
			},
			want: domain.OptionResult{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &optionsProviderTest{variants: tc.variants}
			service := usecase.NewService(provider)

			ctx := t.Context()
			input := usecase.CalculateInput{
				UserID:            "user-1",
				MerchantID:        "shop-42",
				CategoryID:        "groceries",
				PurchaseAmountKop: tc.amount,
			}

			result, err := service.Calculate(ctx, input)
			if err != nil {
				t.Fatalf("Calculate error = %v", err)
			}
			assert.Equal(t, result, tc.want)
			if provider.calls != 1 {
				t.Fatalf("GetRateBPS calls = %d, want 1", provider.calls)
			}
			if provider.ctx != ctx {
				t.Error("GetRateBPS did not receive the original context")
			}
			if provider.input != input {
				t.Errorf("GetRateBPS input = %+v, want %+v", provider.input, input)
			}
		})
	}
}

func TestCalculateErrors(t *testing.T) {
	providerErr := errors.New("options provider unavailable")

	tests := []struct {
		name        string
		amount      int64
		variants    []usecase.BestVariant
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
			name:        "provider_error",
			amount:      100_000,
			providerErr: providerErr,
			wantErr:     providerErr,
			wantCalls:   1,
		},
		{
			name:        "provider_canceled",
			amount:      100_000,
			providerErr: context.Canceled,
			wantErr:     context.Canceled,
			wantCalls:   1,
		},
		{
			name:        "provider_deadline_exceeded",
			amount:      100_000,
			providerErr: context.DeadlineExceeded,
			wantErr:     context.DeadlineExceeded,
			wantCalls:   1,
		},
		{
			name:   "invalid_option_rate",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{RateBPS: 10_001},
			},
			wantErr:   domain.ErrInvalidRate,
			wantCalls: 1,
		},
		{
			name:   "invalid_discount_after_valid_option",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{RateBPS: 100},
				{DiscountKop: -1, RateBPS: 300},
			},
			wantErr:   domain.ErrInvalidDiscount,
			wantCalls: 1,
		},
		{
			name:   "discount_exceeds_purchase",
			amount: 100_000,
			variants: []usecase.BestVariant{
				{DiscountKop: 100_001, RateBPS: 300},
			},
			wantErr:   domain.ErrInvalidDiscount,
			wantCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &optionsProviderTest{
				variants: tc.variants,
				err:      tc.providerErr,
			}
			service := usecase.NewService(provider)

			result, err := service.Calculate(t.Context(), usecase.CalculateInput{
				PurchaseAmountKop: tc.amount,
			})

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Calculate error = %v, want %v", err, tc.wantErr)
			}
			if result != (domain.OptionResult{}) {
				t.Errorf("Calculate result = %+v, want an empty result", result)
			}
			if provider.calls != tc.wantCalls {
				t.Errorf("GetRateBPS calls = %d, want %d", provider.calls, tc.wantCalls)
			}
		})
	}
}

func TestCalculateEmptyOptions(t *testing.T) {
	tests := []struct {
		name     string
		variants []usecase.BestVariant
	}{
		{name: "nil_list"},
		{name: "empty_list", variants: []usecase.BestVariant{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &optionsProviderTest{variants: tc.variants}
			service := usecase.NewService(provider)

			result, err := service.Calculate(t.Context(), usecase.CalculateInput{
				PurchaseAmountKop: 100_000,
			})

			if err == nil {
				t.Fatal("Calculate error = nil, want an error for an empty options list")
			}
			if result != (domain.OptionResult{}) {
				t.Errorf("Calculate result = %+v, want an empty result", result)
			}
			if provider.calls != 1 {
				t.Errorf("GetRateBPS calls = %d, want 1", provider.calls)
			}
		})
	}
}
