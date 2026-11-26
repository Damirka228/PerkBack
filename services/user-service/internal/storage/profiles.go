package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Storage) SaveProfile(ctx context.Context, profile domain.Profile) error {
	const sqlQuery = `
		INSERT INTO user_profiles (user_id, selected_category_ids)
		VALUES ($1, COALESCE($2::text[], '{}'::text[]))
		ON CONFLICT (user_id)
		DO UPDATE SET selected_category_ids = EXCLUDED.selected_category_ids;
	`
	if _, err := s.pool.Exec(ctx, sqlQuery, profile.UserID, profile.SelectedCategoryIDs); err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	return nil
}

func (s *Storage) GetProfile(ctx context.Context, userID string) (domain.Profile, error) {
	const sqlQuery = `
		SELECT selected_category_ids
		FROM user_profiles
		WHERE user_id = $1
	`
	var category []string
	row := s.pool.QueryRow(ctx, sqlQuery, userID)
	err := row.Scan(&category)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.Profile{}, fmt.Errorf("scan profile: %w", err)
	}
	return domain.Profile{
		UserID:              userID,
		SelectedCategoryIDs: category,
	}, nil
}
