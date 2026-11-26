package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/user-service/internal/usecase"
)

func TestGetProfileReturnsRepositoryResult(t *testing.T) {
	for _, tt := range []struct {
		name       string
		categories []string
	}{
		{"selected_categories", []string{"transport", "food"}},
		{"five_categories", []string{"food", "transport", "travel", "books", "sports"}},
		{"empty_selection", []string{}},
		{"nil_selection", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := domain.Profile{UserID: "user-42", SelectedCategoryIDs: tt.categories}
			repository := &fakeProfileRepository{profile: want}
			service := usecase.NewService(repository)
			ctx := t.Context()

			got, err := service.GetProfile(ctx, want.UserID)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("GetProfile() = (%+v, %v), want (%+v, nil)", got, err, want)
			}
			assertProfileRead(t, repository, ctx, want.UserID)
		})
	}
}

func TestGetProfileRejectsInvalidUserBeforeCallingRepository(t *testing.T) {
	for _, tt := range []struct {
		name   string
		userID string
	}{
		{"empty", ""},
		{"whitespace", " \t\n"},
		{"unicode_whitespace", "\u00a0\u2003"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeProfileRepository{}
			service := usecase.NewService(repository)
			got, err := service.GetProfile(t.Context(), tt.userID)
			if !errors.Is(err, domain.ErrUserIDEmpty) || !reflect.DeepEqual(got, domain.Profile{}) {
				t.Fatalf("GetProfile() = (%+v, %v), want empty profile and ErrUserIDEmpty", got, err)
			}
			if len(repository.getCalls) != 0 || len(repository.calls) != 0 {
				t.Errorf("invalid input called repository: reads=%d, saves=%d",
					len(repository.getCalls), len(repository.calls))
			}
		})
	}
}

func TestGetProfilePreservesRepositoryError(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"not_found", domain.ErrProfileNotFound},
		{"database_error", errors.New("profile storage unavailable")},
		{"canceled", context.Canceled},
		{"deadline", context.DeadlineExceeded},
	} {
		for _, wrapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wrapped=%t", tt.name, wrapped), func(t *testing.T) {
				repositoryErr := tt.err
				if wrapped {
					repositoryErr = fmt.Errorf("repository read: %w", repositoryErr)
				}
				repository := &fakeProfileRepository{
					// An error must not return a partially populated profile.
					profile: domain.Profile{UserID: "user-42", SelectedCategoryIDs: []string{"food"}},
					getErr:  repositoryErr,
				}
				service := usecase.NewService(repository)
				ctx := t.Context()

				got, err := service.GetProfile(ctx, "user-42")
				if !errors.Is(err, tt.err) || !reflect.DeepEqual(got, domain.Profile{}) {
					t.Fatalf("GetProfile() = (%+v, %v), want empty profile and wrapped %v", got, err, tt.err)
				}
				assertProfileRead(t, repository, ctx, "user-42")
			})
		}
	}
}

func assertProfileRead(t *testing.T, repository *fakeProfileRepository, ctx context.Context, userID string) {
	t.Helper()
	if len(repository.getCalls) != 1 {
		t.Fatalf("GetProfile calls = %d, want 1", len(repository.getCalls))
	}
	call := repository.getCalls[0]
	if call.ctx != ctx || call.userID != userID {
		t.Errorf("GetProfile must receive the original context and user ID %q; got ID %q", userID, call.userID)
	}
	if len(repository.calls) != 0 {
		t.Errorf("GetProfile called SaveProfile %d times, want 0", len(repository.calls))
	}
}
