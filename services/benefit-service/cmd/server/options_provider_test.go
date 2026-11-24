package main

import (
	"errors"
	"fmt"
	"slices"
	"sync"
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
	service := usecase.NewService(provider, nil, nil)
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

func TestOptionsProviderUpsert(t *testing.T) {
	rule := domain.RulePromotion{
		ID:                   "promotion-1",
		ShopID:               "shop-42",
		CategoryID:           "groceries",
		MinPurchaseAmountKop: 100_000,
		DiscountKop:          7_000,
		RateBPS:              300,
	}
	other := rule
	other.ID = "promotion-2"
	other.DiscountKop = 2_000
	old := domain.RulePromotion{
		ID:                   rule.ID,
		ShopID:               "old-shop",
		CategoryID:           "old-category",
		MinPurchaseAmountKop: 200_000,
		DiscountKop:          1_000,
		RateBPS:              100,
	}
	invalid := rule
	invalid.ID = ""

	tests := []struct {
		name        string
		initial     []domain.RulePromotion
		rule        domain.RulePromotion
		repetitions int
		want        []domain.RulePromotion
		wantErr     error
	}{
		{
			name: "add_to_empty_provider", rule: rule, repetitions: 1,
			want: []domain.RulePromotion{rule},
		},
		{
			name:    "append_new_id_without_replacing_existing_rule",
			initial: []domain.RulePromotion{other}, rule: rule, repetitions: 1,
			want: []domain.RulePromotion{rule, other},
		},
		{
			name:    "replace_all_fields_of_matching_id_and_preserve_order",
			initial: []domain.RulePromotion{other, old}, rule: rule, repetitions: 1,
			want: []domain.RulePromotion{other, rule},
		},
		{
			name:    "repeated_event_does_not_duplicate_rule",
			initial: []domain.RulePromotion{other}, rule: rule, repetitions: 3,
			want: []domain.RulePromotion{rule, other},
		},
		{
			name:    "empty_id_does_not_modify_existing_rules",
			initial: []domain.RulePromotion{other, old}, rule: invalid, repetitions: 1,
			want: []domain.RulePromotion{other, old}, wantErr: ErrRuleIdEmpty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &OptionsProvider{rateBPS: 100, rules: slices.Clone(tt.initial)}
			for i := 0; i < tt.repetitions; i++ {
				if err := provider.Upsert(tt.rule); !errors.Is(err, tt.wantErr) {
					t.Fatalf("Upsert() error = %v, want %v", err, tt.wantErr)
				}
			}
			// No concurrent operations in this test: compare every field,
			// the number of rules, and their order after Upsert returns.
			if !slices.Equal(provider.rules, tt.want) {
				t.Fatalf("rules = %+v, want %+v", provider.rules, tt.want)
			}
		})
	}
}

func TestOptionsProviderUpsertUpdatesCalculatedOptions(t *testing.T) {
	provider := &OptionsProvider{rateBPS: 100}
	input := usecase.CalculateInput{
		ShopID:            "shop-42",
		CategoryID:        "groceries",
		PurchaseAmountKop: 100_000,
	}
	rule := domain.RulePromotion{
		ID:                   "promotion-1",
		ShopID:               input.ShopID,
		CategoryID:           input.CategoryID,
		MinPurchaseAmountKop: input.PurchaseAmountKop,
		DiscountKop:          7_000,
		RateBPS:              300,
	}
	if err := provider.Upsert(rule); err != nil {
		t.Fatal(err)
	}
	rule.DiscountKop = 9_000
	rule.RateBPS = 500
	if err := provider.Upsert(rule); err != nil {
		t.Fatal(err)
	}
	got, err := provider.GetOptions(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	want := []usecase.BestVariant{
		{RateBPS: 100},
		{DiscountKop: 9_000, RateBPS: 500},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("GetOptions() after replacement = %+v, want %+v", got, want)
	}
}

func TestOptionsProviderConcurrentUpsertAndGetOptions(t *testing.T) {
	const writers, readers, iterations = 4, 4, 200
	provider := &OptionsProvider{rateBPS: 100}
	input := usecase.CalculateInput{
		ShopID:            "shop-42",
		CategoryID:        "groceries",
		PurchaseAmountKop: 100_000,
	}
	ctx := t.Context()
	start := make(chan struct{})
	var wg sync.WaitGroup

	for writer := 0; writer < writers; writer++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				rule := domain.RulePromotion{
					ID:                   fmt.Sprintf("promotion-%d", id),
					ShopID:               input.ShopID,
					CategoryID:           input.CategoryID,
					MinPurchaseAmountKop: input.PurchaseAmountKop,
					DiscountKop:          int64(i),
					RateBPS:              int64(i),
				}
				if err := provider.Upsert(rule); err != nil {
					t.Errorf("concurrent Upsert(): %v", err)
					return
				}
			}
		}(writer)
	}
	for reader := 0; reader < readers; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				options, err := provider.GetOptions(ctx, input)
				if err != nil {
					t.Errorf("concurrent GetOptions(): %v", err)
					return
				}
				if len(options) < 1 || len(options) > writers+1 {
					t.Errorf("option count = %d, want base plus at most %d rules", len(options), writers)
					return
				}
				if options[0] != (usecase.BestVariant{RateBPS: 100}) {
					t.Errorf("base option changed: %+v", options[0])
					return
				}
				for _, option := range options[1:] {
					// Each writer updates both fields to the same value.
					// Readers must never observe a partially updated rule.
					if option.DiscountKop != option.RateBPS {
						t.Errorf("inconsistent rule snapshot: %+v", option)
						return
					}
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(provider.rules) != writers {
		t.Fatalf("rule count after concurrent updates = %d, want %d", len(provider.rules), writers)
	}
	seen := make(map[string]bool)
	for _, rule := range provider.rules {
		if seen[rule.ID] {
			t.Fatalf("duplicate rule ID after concurrent updates: %s", rule.ID)
		}
		seen[rule.ID] = true
		if rule.DiscountKop != iterations-1 || rule.RateBPS != iterations-1 {
			t.Fatalf("last update was not preserved: %+v", rule)
		}
	}
	for writer := 0; writer < writers; writer++ {
		if id := fmt.Sprintf("promotion-%d", writer); !seen[id] {
			t.Errorf("missing rule after concurrent updates: %s", id)
		}
	}
}
