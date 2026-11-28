package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

func TestApplyCategoryRate(t *testing.T) {
	for _, tt := range []struct {
		name         string
		rate         int64
		categoryRate int64
		selected     bool
		want         int64
	}{
		{"not_selected_keeps_original_rate", 100, 500, false, 100},
		{"selected_raises_base_rate", 100, 500, true, 500},
		{"selected_raises_promotion_rate", 300, 500, true, 500},
		{"selected_preserves_better_promotion", 700, 500, true, 700},
		{"equal_rates_are_not_added", 500, 500, true, 500},
		{"both_zero", 0, 0, true, 0},
		{"zero_original_rate_selected", 0, 500, true, 500},
		{"zero_original_rate_not_selected", 0, 500, false, 0},
		{"zero_category_rate_keeps_original", 300, 0, true, 300},
		{"original_upper_boundary", 10000, 500, true, 10000},
		{"category_upper_boundary_selected", 500, 10000, true, 10000},
		{"category_upper_boundary_not_selected", 500, 10000, false, 500},
		{"both_upper_boundary", 10000, 10000, true, 10000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ApplyCategoryRate(tt.rate, tt.categoryRate, tt.selected)
			if err != nil || got != tt.want {
				t.Fatalf("ApplyCategoryRate(%d, %d, %t) = (%d, %v), want (%d, nil)",
					tt.rate, tt.categoryRate, tt.selected, got, err, tt.want)
			}
		})
	}
}

func TestApplyCategoryRateRejectsInvalidRates(t *testing.T) {
	for _, tt := range []struct {
		name         string
		rate         int64
		categoryRate int64
	}{
		{"negative_original", -1, 500},
		{"original_above_limit", 10001, 500},
		{"negative_category", 100, -1},
		{"category_above_limit", 100, 10001},
	} {
		for _, selected := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/selected=%t", tt.name, selected), func(t *testing.T) {
				amount, err := domain.ApplyCategoryRate(tt.rate, tt.categoryRate, selected)
				if amount != 0 || !errors.Is(err, domain.ErrInvalidRate) {
					t.Fatalf("ApplyCategoryRate() = (%d, %v), want (0, ErrInvalidRate)", amount, err)
				}
			})
		}
	}
}

func TestApplyCategoryRateCashbackAfterDiscount(t *testing.T) {
	rate, err := domain.ApplyCategoryRate(300, 500, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := domain.CalculateOptions(100_000, 7_000, rate)
	want := domain.OptionResult{DiscountKop: 7_000, PayableKop: 93_000, CashbackKop: 4_650}
	if err != nil || got != want {
		t.Fatalf("CalculateOptions with category rate = (%+v, %v), want (%+v, nil)", got, err, want)
	}
}
