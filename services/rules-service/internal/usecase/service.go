package usecase

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
)

type PromotionRepository interface {
	GetAllPromotions(ctx context.Context) ([]domain.Promotion, error)
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
