package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/user-service/internal/usecase"
)

type saveProfileCall struct {
	ctx     context.Context
	profile domain.Profile
}

type fakeProfileRepository struct {
	calls []saveProfileCall
	err   error
}

func (f *fakeProfileRepository) SaveProfile(ctx context.Context, profile domain.Profile) error {
	profile.SelectedCategoryIDs = slices.Clone(profile.SelectedCategoryIDs)
	f.calls = append(f.calls, saveProfileCall{ctx, profile})
	return f.err
}

func TestSetSelectedCategoriesSavesValidProfile(t *testing.T) {
	for _, tt := range []struct {
		name       string
		categories []string
	}{
		{"selected_categories", []string{"transport", "food"}},
		{"five_categories", []string{"food", "transport", "travel", "books", "sports"}},
		{"nil_clears_selection", nil},
		{"empty_slice_clears_selection", []string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeProfileRepository{}
			service := usecase.NewService(repository)
			ctx := t.Context()
			original := slices.Clone(tt.categories)

			if err := service.SetSelectedCategories(ctx, "user-1", tt.categories); err != nil {
				t.Fatalf("SetSelectedCategories(): %v", err)
			}
			assertProfileSaved(t, repository, ctx, domain.Profile{
				UserID: "user-1", SelectedCategoryIDs: tt.categories,
			})
			if !slices.Equal(tt.categories, original) {
				t.Errorf("SetSelectedCategories modified input: got %v, want %v", tt.categories, original)
			}
		})
	}
}

func TestSetSelectedCategoriesRejectsInvalidProfileBeforeSaving(t *testing.T) {
	for _, tt := range []struct {
		name       string
		userID     string
		categories []string
		wantErr    error
	}{
		{"empty_user", "", []string{"food"}, domain.ErrUserIDEmpty},
		{"whitespace_user", " \t\n", nil, domain.ErrUserIDEmpty},
		{"empty_category", "user-1", []string{""}, domain.ErrCategoryIDEmpty},
		{"whitespace_category", "user-1", []string{"food", " \t\n"}, domain.ErrCategoryIDEmpty},
		{"duplicate_category", "user-1", []string{"food", "transport", "food"}, domain.ErrDuplicateCategory},
		{
			"six_categories", "user-1",
			[]string{"food", "transport", "travel", "books", "sports", "health"},
			domain.ErrTooManyCategories,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeProfileRepository{}
			service := usecase.NewService(repository)
			err := service.SetSelectedCategories(t.Context(), tt.userID, tt.categories)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("SetSelectedCategories() = %v, want wrapped %v", err, tt.wantErr)
			}
			if len(repository.calls) != 0 {
				t.Errorf("SaveProfile calls = %d, want 0 for invalid input", len(repository.calls))
			}
		})
	}
}

func TestSetSelectedCategoriesPreservesRepositoryError(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"storage_failure", errors.New("profile storage unavailable")},
		{"canceled", context.Canceled},
		{"deadline", context.DeadlineExceeded},
	} {
		for _, wrapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wrapped=%t", tt.name, wrapped), func(t *testing.T) {
				repositoryErr := tt.err
				if wrapped {
					repositoryErr = fmt.Errorf("repository operation: %w", repositoryErr)
				}
				repository := &fakeProfileRepository{err: repositoryErr}
				service := usecase.NewService(repository)
				ctx := t.Context()
				profile := domain.Profile{UserID: "user-1", SelectedCategoryIDs: []string{"food"}}

				err := service.SetSelectedCategories(ctx, profile.UserID, profile.SelectedCategoryIDs)
				if !errors.Is(err, tt.err) {
					t.Errorf("SetSelectedCategories() = %v, want wrapped %v", err, tt.err)
				}
				assertProfileSaved(t, repository, ctx, profile)
			})
		}
	}
}

func TestSetSelectedCategoriesPassesReplacementRatherThanMergedSelection(t *testing.T) {
	repository := &fakeProfileRepository{}
	service := usecase.NewService(repository)
	ctx := t.Context()
	for _, categories := range [][]string{
		{"food", "transport"},
		{"books"},
		nil,
	} {
		repository.calls = nil
		if err := service.SetSelectedCategories(ctx, "user-1", categories); err != nil {
			t.Fatalf("SetSelectedCategories(%v): %v", categories, err)
		}
		assertProfileSaved(t, repository, ctx, domain.Profile{
			UserID: "user-1", SelectedCategoryIDs: categories,
		})
	}
}

func assertProfileSaved(t *testing.T, repository *fakeProfileRepository, ctx context.Context, want domain.Profile) {
	t.Helper()
	if len(repository.calls) != 1 {
		t.Fatalf("SaveProfile calls = %d, want 1", len(repository.calls))
	}
	call := repository.calls[0]
	if call.ctx != ctx {
		t.Error("SaveProfile did not receive the original context")
	}
	if call.profile.UserID != want.UserID || !slices.Equal(call.profile.SelectedCategoryIDs, want.SelectedCategoryIDs) {
		t.Errorf("SaveProfile profile = %+v, want %+v", call.profile, want)
	}
}
