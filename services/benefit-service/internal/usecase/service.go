package usecase

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

type OptionsProvider interface {
	GetRateBPS(ctx context.Context, input CalculateInput) ([]BestVariant, error)
}

type Service struct {
	optionsProvider OptionsProvider
}

func NewService(optionsProvider OptionsProvider) *Service {
	return &Service{
		optionsProvider: optionsProvider,
	}
}

func (s *Service) Calculate(ctx context.Context, input CalculateInput) (domain.OptionResult, error) {
	if input.PurchaseAmountKop < 0 {
		return domain.OptionResult{}, domain.ErrInvalidAmount
	}
	variants, err := s.optionsProvider.GetRateBPS(ctx, input)
	if err != nil {
		return domain.OptionResult{}, fmt.Errorf("error get list variant:%w", err)
	}

	if len(variants) == 0 {
		return domain.OptionResult{}, fmt.Errorf("variants list is empty")
	}

	var maxBenefit int64
	var bestResult *domain.OptionResult
	var isFirst bool = true

	for _, variant := range variants {
		res, err := domain.CalculateOptions(input.PurchaseAmountKop, variant.DiscountKop, variant.RateBPS)
		if err != nil {
			return domain.OptionResult{}, fmt.Errorf("err get variant for best variants: %w", err)
		}
		currBenefit := res.DiscountKop + res.CashbackKop
		if isFirst {
			maxBenefit = currBenefit
			bestResult = &res
			isFirst = false
			continue
		}
		if currBenefit > maxBenefit {
			maxBenefit = currBenefit
			bestResult = &res
		}
	}
	return *bestResult, nil
}
