package storage

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
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
