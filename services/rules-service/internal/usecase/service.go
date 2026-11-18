package usecase

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
)

type PromotionRepository interface {
	GetAllPromotions(ctx context.Context) ([]domain.Promotion, error)
	CreatePromotion(ctx context.Context, promotion domain.Promotion) error
}

type Service struct {
	promotionRepository PromotionRepository
}

func NewService(repo PromotionRepository) *Service {
	return &Service{
		promotionRepository: repo,
	}
}

func (s *Service) GetAllPromotions(ctx context.Context) ([]domain.Promotion, error) {
	promotions, err := s.promotionRepository.GetAllPromotions(ctx)
	if err != nil {
		return nil, fmt.Errorf("err get promotions:%w", err)
	}
	return promotions, nil
}

func (s *Service) CreatePromotion(ctx context.Context, promotion domain.Promotion) error {
	if err := promotion.Validate(); err != nil {
		return fmt.Errorf("err validate promotions:%w", err)
	}
	if err := s.promotionRepository.CreatePromotion(ctx, promotion); err != nil {
		return fmt.Errorf("err to get promotions to repo:%w", err)
	}
	return nil
}
