package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
)

func TestProfileValidate(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		categories []string
		wantErr    error
	}{
		{name: "valid_profile", userID: "user-1", categories: []string{"food", "transport"}},
		{name: "nil_categories", userID: "user-1"},
		{name: "empty_categories", userID: "user-1", categories: []string{}},
		{name: "single_category", userID: "user-1", categories: []string{"food"}},
		{name: "category_order_does_not_matter", userID: "user-1", categories: []string{"transport", "food"}},
		{
			name: "five_categories_allowed", userID: "user-1",
			categories: []string{"food", "transport", "travel", "books", "sports"},
		},
		{
			name: "six_categories_rejected", userID: "user-1",
			categories: []string{"food", "transport", "travel", "books", "sports", "health"},
			wantErr:    domain.ErrTooManyCategories,
		},
		{name: "empty_user", categories: []string{"food"}, wantErr: domain.ErrUserIDEmpty},
		{name: "whitespace_user", userID: " \t\n", wantErr: domain.ErrUserIDEmpty},
		{name: "unicode_whitespace_user", userID: "\u00a0\u2003", wantErr: domain.ErrUserIDEmpty},
		{
			name: "empty_category", userID: "user-1",
			categories: []string{""}, wantErr: domain.ErrCategoryIDEmpty,
		},
		{
			name: "whitespace_category", userID: "user-1",
			categories: []string{" \t\n"}, wantErr: domain.ErrCategoryIDEmpty,
		},
		{
			name: "unicode_whitespace_category", userID: "user-1",
			categories: []string{"\u00a0\u2003"}, wantErr: domain.ErrCategoryIDEmpty,
		},
		{
			name: "empty_category_after_valid_one", userID: "user-1",
			categories: []string{"food", ""}, wantErr: domain.ErrCategoryIDEmpty,
		},
		{
			name: "adjacent_duplicate", userID: "user-1",
			categories: []string{"food", "food"}, wantErr: domain.ErrDuplicateCategory,
		},
		{
			name: "non_adjacent_duplicate", userID: "user-1",
			categories: []string{"food", "transport", "food"}, wantErr: domain.ErrDuplicateCategory,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := domain.Profile{
				UserID: tt.userID, SelectedCategoryIDs: slices.Clone(tt.categories),
			}
			originalCategories := slices.Clone(profile.SelectedCategoryIDs)
			if err := profile.Validate(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
			if profile.UserID != tt.userID ||
				!slices.Equal(profile.SelectedCategoryIDs, originalCategories) ||
				(profile.SelectedCategoryIDs == nil) != (originalCategories == nil) {
				t.Errorf("Validate() modified the profile: %+v", profile)
			}
		})
	}
}
