package storage_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSaveProfileCreatesReplacesAndRepeatsIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	other := domain.Profile{UserID: "user-2", SelectedCategoryIDs: []string{"health"}}
	if err := fixture.SaveProfile(ctx, other); err != nil {
		t.Fatalf("save another user's profile: %v", err)
	}
	assertProfiles(t, fixture, other)

	for _, step := range []struct {
		name       string
		categories []string
	}{
		{"create", []string{"transport", "food"}},
		{"replace_not_append", []string{"books"}},
		{"five_categories", []string{"food", "transport", "travel", "books", "sports"}},
		{"repeat_does_not_duplicate", []string{"food", "transport", "travel", "books", "sports"}},
	} {
		profile := domain.Profile{UserID: "user-1", SelectedCategoryIDs: step.categories}
		if err := fixture.SaveProfile(ctx, profile); err != nil {
			t.Fatalf("%s: SaveProfile(): %v", step.name, err)
		}
		// Verify exact category order, row count and isolation between users.
		assertProfiles(t, fixture, profile, other)
	}
}

func TestSaveProfileEmptySelectionIsAnArrayNotNullIntegration(t *testing.T) {
	for _, tt := range []struct {
		name       string
		categories []string
		existing   bool
	}{
		{"create_nil", nil, false},
		{"create_empty_slice", []string{}, false},
		{"clear_with_nil", nil, true},
		{"clear_with_empty_slice", []string{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			ctx := storageTestContext(t)
			if tt.existing {
				if err := fixture.SaveProfile(ctx, domain.Profile{
					UserID: "user-1", SelectedCategoryIDs: []string{"food", "transport"},
				}); err != nil {
					t.Fatalf("seed profile: %v", err)
				}
			}
			profile := domain.Profile{UserID: "user-1", SelectedCategoryIDs: tt.categories}
			for range 2 {
				if err := fixture.SaveProfile(ctx, profile); err != nil {
					t.Fatalf("save empty selection: %v", err)
				}
				assertProfiles(t, fixture, domain.Profile{UserID: "user-1", SelectedCategoryIDs: []string{}})
				var isNull bool
				var count int
				if err := fixture.pool.QueryRow(ctx, `
					SELECT selected_category_ids IS NULL, cardinality(selected_category_ids)
					FROM user_profiles WHERE user_id = $1
				`, profile.UserID).Scan(&isNull, &count); err != nil {
					t.Fatalf("inspect empty selection: %v", err)
				}
				if isNull || count != 0 {
					t.Fatalf("empty selection: isNull=%t, count=%d, want false and 0", isNull, count)
				}
			}
		})
	}
}

func TestSaveProfileDatabaseRejectsSixCategoriesIntegration(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new_profile"
		if existing {
			name = "existing_profile"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newIntegrationStorage(t)
			ctx := storageTestContext(t)
			var want []domain.Profile
			if existing {
				want = []domain.Profile{{UserID: "user-1", SelectedCategoryIDs: []string{"food"}}}
				if err := fixture.SaveProfile(ctx, want[0]); err != nil {
					t.Fatalf("seed profile: %v", err)
				}
			}

			// Deliberately bypass usecase validation to exercise PostgreSQL's
			// CHECK on both the INSERT and ON CONFLICT DO UPDATE paths.
			err := fixture.SaveProfile(ctx, domain.Profile{
				UserID: "user-1",
				SelectedCategoryIDs: []string{
					"food", "transport", "travel", "books", "sports", "health",
				},
			})
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("SaveProfile() = %v, want wrapped check constraint violation", err)
			}
			assertProfiles(t, fixture, want...)

			// A failed statement must not leave the single-connection storage
			// unusable or partially replace the original categories.
			retry := domain.Profile{UserID: "user-1", SelectedCategoryIDs: []string{"transport"}}
			if err := fixture.SaveProfile(ctx, retry); err != nil {
				t.Fatalf("save valid profile after constraint error: %v", err)
			}
			assertProfiles(t, fixture, retry)
		})
	}
}

func TestSaveProfileCanceledContextDoesNotChangeProfileIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	original := domain.Profile{UserID: "user-1", SelectedCategoryIDs: []string{"food"}}
	if err := fixture.SaveProfile(storageTestContext(t), original); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := fixture.SaveProfile(ctx, domain.Profile{
		UserID: original.UserID, SelectedCategoryIDs: []string{"transport"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SaveProfile() = %v, want wrapped context.Canceled", err)
	}
	assertProfiles(t, fixture, original)
}

func assertProfiles(t *testing.T, fixture *storageFixture, want ...domain.Profile) {
	t.Helper()
	// Independent pool reads only committed state. Compare every row so
	// duplicate profiles and accidental updates of other users are detected.
	rows, err := fixture.pool.Query(storageTestContext(t), `
		SELECT user_id, selected_category_ids FROM user_profiles ORDER BY user_id
	`)
	if err != nil {
		t.Fatalf("query profiles: %v", err)
	}
	profiles, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Profile])
	if err != nil {
		t.Fatalf("read profiles: %v", err)
	}
	if len(profiles) != len(want) {
		t.Fatalf("profile count = %d, want %d; profiles=%+v", len(profiles), len(want), profiles)
	}
	for i, profile := range profiles {
		if profile.UserID != want[i].UserID || !slices.Equal(profile.SelectedCategoryIDs, want[i].SelectedCategoryIDs) {
			t.Errorf("profile[%d] = %+v, want %+v", i, profile, want[i])
		}
	}
}
