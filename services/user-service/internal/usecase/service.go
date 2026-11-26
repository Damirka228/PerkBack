package usecase

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
)

type ProfileRepository interface {
	SaveProfile(ctx context.Context, profile domain.Profile) error
}

type Service struct {
	profileRepository ProfileRepository
}

func NewService(profileRepository ProfileRepository) *Service {
	return &Service{
		profileRepository: profileRepository,
	}
}

func (s *Service) SetSelectedCategories(ctx context.Context, userID string, categoryIDs []string) error {
	profile := domain.Profile{
		UserID:              userID,
		SelectedCategoryIDs: categoryIDs,
	}
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validate profile: %w", err)
	}
	if err := s.profileRepository.SaveProfile(ctx, profile); err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	return nil
}
