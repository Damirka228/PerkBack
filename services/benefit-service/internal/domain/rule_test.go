package domain

import "testing"

func TestRulePromotionMatchesRule(t *testing.T) {
	rule := RulePromotion{
		ShopID:               "shop-42",
		CategoryID:           "groceries",
		MinPurchaseAmountKop: 100_000,
		DiscountKop:          7_000,
		RateBPS:              300,
	}

	tests := []struct {
		name       string
		shopID     string
		categoryID string
		amount     int64
		want       bool
	}{
		{
			name:       "at_threshold",
			shopID:     "shop-42",
			categoryID: "groceries",
			amount:     100_000,
			want:       true,
		},
		{
			name:       "above_threshold",
			shopID:     "shop-42",
			categoryID: "groceries",
			amount:     100_001,
			want:       true,
		},
		{
			name:       "below_threshold",
			shopID:     "shop-42",
			categoryID: "groceries",
			amount:     99_999,
		},
		{
			name:       "different_shop",
			shopID:     "shop-other",
			categoryID: "groceries",
			amount:     100_000,
		},
		{
			name:       "different_category",
			shopID:     "shop-42",
			categoryID: "electronics",
			amount:     100_000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rule.MatchesRule(tc.shopID, tc.categoryID, tc.amount)
			if got != tc.want {
				t.Errorf("MatchesRule(%q, %q, %d) = %t, want %t",
					tc.shopID, tc.categoryID, tc.amount, got, tc.want)
			}
		})
	}
}
