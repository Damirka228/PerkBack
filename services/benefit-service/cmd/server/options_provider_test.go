package main

import (
	"slices"
	"testing"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
)

func TestOptionsProviderGetOptions(t *testing.T) {
	promotion := domain.RulePromotion{
		ShopID:               "shop-42",
		CategoryID:           "groceries",
		MinPurchaseAmountKop: 100_000,
		DiscountKop:          7_000,
		RateBPS:              300,
	}
	otherShop := promotion
	otherShop.ShopID = "shop-other"
	highThreshold := promotion
	highThreshold.MinPurchaseAmountKop = 200_000
	secondPromotion := promotion
	secondPromotion.DiscountKop = 2_000
	secondPromotion.RateBPS = 500

	base := usecase.BestVariant{RateBPS: 100}
	promotionOption := usecase.BestVariant{DiscountKop: 7_000, RateBPS: 300}

	tests := []struct {
		name       string
		rules      []domain.RulePromotion
		shopID     string
		categoryID string
		amount     int64
		want       []usecase.BestVariant
	}{
		{
			name:       "no_rules_returns_base_option",
			shopID:     "shop-42",
			categoryID: "groceries",
			amount:     100_000,
			want:       []usecase.BestVariant{base},
		},
		{
			name:       "different_shop_returns_base_option",
			rules:      []domain.RulePromotion{promotion},
			shopID:     "shop-other",
			categoryID: "groceries",
			amount:     100_000,
			want:       []usecase.BestVariant{base},
		},
		{
			name:       "different_category_returns_base_option",
			rules:      []domain.RulePromotion{promotion},
			shopID:     "shop-42",
			categoryID: "electronics",
			amount:     100_000,
			want:       []usecase.BestVariant{base},
		},
		{
			name:       "below_threshold_returns_base_option",
			rules:      []domain.RulePromotion{promotion},
			shopID:     "shop-42",
			categoryID: "groceries",
			amount:     99_999,
			want:       []usecase.BestVariant{base},
		},
		{
			name:       "threshold_includes_promotion_with_its_terms",
			rules:      []domain.RulePromotion{promotion},
			shopID:     "shop-42",
			categoryID: "groceries",
			amount:     100_000,
			want:       []usecase.BestVariant{base, promotionOption},
		},
		{
			name:       "returns_all_matching_promotions_in_order",
			rules:      []domain.RulePromotion{promotion, otherShop, secondPromotion, highThreshold},
			shopID:     "shop-42",
			categoryID: "groceries",
			amount:     100_000,
			want: []usecase.BestVariant{
				base,
				promotionOption,
				{DiscountKop: 2_000, RateBPS: 500},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &OptionsProvider{rateBPS: 100, rules: tc.rules}
			input := usecase.CalculateInput{
				UserID:            "user-1",
				ShopID:            tc.shopID,
				CategoryID:        tc.categoryID,
				PurchaseAmountKop: tc.amount,
			}

			got, err := provider.GetOptions(t.Context(), input)
			if err != nil {
				t.Fatalf("GetOptions error = %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("GetOptions = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCalculateWithMatchingRule(t *testing.T) {
	provider := &OptionsProvider{
		rateBPS: 100,
		rules: []domain.RulePromotion{
			{
				ShopID:               "shop-42",
				CategoryID:           "groceries",
				MinPurchaseAmountKop: 100_000,
				DiscountKop:          7_000,
				RateBPS:              300,
			},
		},
	}
	service := usecase.NewService(provider)
	input := usecase.CalculateInput{
		UserID:            "user-1",
		ShopID:            "shop-42",
		CategoryID:        "groceries",
		PurchaseAmountKop: 100_000,
	}

	got, err := service.Calculate(t.Context(), input)
	if err != nil {
		t.Fatalf("Calculate error = %v", err)
	}
	want := domain.OptionResult{
		DiscountKop: 7_000,
		PayableKop:  93_000,
		CashbackKop: 2_790,
	}
	if got != want {
		t.Errorf("Calculate = %+v, want %+v", got, want)
	}
}
