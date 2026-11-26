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

func TestGetProfileReadsRequestedUserWithoutWritesIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	profiles := []domain.Profile{
		{UserID: "user-1", SelectedCategoryIDs: []string{"transport", "food", "travel", "books", "sports"}},
		{UserID: "user-2", SelectedCategoryIDs: []string{"health"}},
		{UserID: "user-3", SelectedCategoryIDs: []string{}},
	}
	for _, profile := range profiles {
		if err := fixture.SaveProfile(ctx, profile); err != nil {
			t.Fatalf("seed profile %q: %v", profile.UserID, err)
		}
	}

	for _, want := range profiles {
		for range 2 {
			got, err := fixture.GetProfile(ctx, want.UserID)
			if err != nil || got.UserID != want.UserID || !slices.Equal(got.SelectedCategoryIDs, want.SelectedCategoryIDs) {
				t.Fatalf("GetProfile(%q) = (%+v, %v), want (%+v, nil)", want.UserID, got, err, want)
			}
			assertProfiles(t, fixture, profiles...)
		}
	}
	got, err := fixture.GetProfile(ctx, "missing-user")
	assertProfileNotFound(t, got, err)
	// Not finding a user must not create a new profile.
	assertProfiles(t, fixture, profiles...)
}

func TestGetProfileMissingInEmptyDatabaseDoesNotCreateRowsIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	got, err := fixture.GetProfile(storageTestContext(t), "missing-user")
	assertProfileNotFound(t, got, err)
	assertProfiles(t, fixture)
}

func TestGetProfileSeesReplacementAndClearedSelectionIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	for _, categories := range [][]string{
		{"food", "transport"},
		{"books"},
		nil,
		{"travel"},
		{},
	} {
		profile := domain.Profile{UserID: "user-1", SelectedCategoryIDs: categories}
		if err := fixture.SaveProfile(ctx, profile); err != nil {
			t.Fatalf("save profile: %v", err)
		}
		got, err := fixture.GetProfile(ctx, profile.UserID)
		if err != nil || got.UserID != profile.UserID || !slices.Equal(got.SelectedCategoryIDs, categories) {
			t.Fatalf("GetProfile() = (%+v, %v), want saved selection %v", got, err, categories)
		}
		// A saved empty array is an existing profile, not ErrProfileNotFound.
		assertProfiles(t, fixture, profile)
	}
}

func TestGetProfilePreservesDatabaseErrorIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	ctx := storageTestContext(t)
	original := domain.Profile{UserID: "user-1", SelectedCategoryIDs: []string{"food"}}
	if err := fixture.SaveProfile(ctx, original); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	// Inject a SELECT failure only inside this test's disposable schema.
	if _, err := fixture.pool.Exec(ctx, `
		ALTER TABLE user_profiles RENAME COLUMN selected_category_ids TO saved_category_ids
	`); err != nil {
		t.Fatal(err)
	}
	got, err := fixture.GetProfile(ctx, original.UserID)
	var pgErr *pgconn.PgError
	if got.UserID != "" || got.SelectedCategoryIDs != nil || !errors.As(err, &pgErr) || pgErr.Code != "42703" {
		t.Fatalf("GetProfile() = (%+v, %v), want empty profile and wrapped undefined-column error", got, err)
	}
	if errors.Is(err, domain.ErrProfileNotFound) {
		t.Error("database failure must not be reported as a missing profile")
	}

	if _, err := fixture.pool.Exec(ctx, `
		ALTER TABLE user_profiles RENAME COLUMN saved_category_ids TO selected_category_ids
	`); err != nil {
		t.Fatal(err)
	}
	got, err = fixture.GetProfile(ctx, original.UserID)
	if err != nil || got.UserID != original.UserID || !slices.Equal(got.SelectedCategoryIDs, original.SelectedCategoryIDs) {
		t.Fatalf("read after restoring column = (%+v, %v), want original profile", got, err)
	}
	assertProfiles(t, fixture, original)
}

func TestGetProfileCanceledContextDoesNotChangeRowsIntegration(t *testing.T) {
	fixture := newIntegrationStorage(t)
	original := domain.Profile{UserID: "user-1", SelectedCategoryIDs: []string{"food"}}
	if err := fixture.SaveProfile(storageTestContext(t), original); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := fixture.GetProfile(ctx, original.UserID)
	if got.UserID != "" || got.SelectedCategoryIDs != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("GetProfile() = (%+v, %v), want empty profile and wrapped context.Canceled", got, err)
	}
	if errors.Is(err, domain.ErrProfileNotFound) {
		t.Error("context cancellation must not be reported as a missing profile")
	}
	assertProfiles(t, fixture, original)
}

func assertProfileNotFound(t *testing.T, profile domain.Profile, err error) {
	t.Helper()
	if profile.UserID != "" || profile.SelectedCategoryIDs != nil || !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("GetProfile() = (%+v, %v), want empty profile and ErrProfileNotFound", profile, err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Error("GetProfile leaked pgx.ErrNoRows instead of the domain error")
	}
}
