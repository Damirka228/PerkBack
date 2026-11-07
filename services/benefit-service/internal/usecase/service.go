package usecase

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

type RateProvider interface {
	GetRateBPS(ctx context.Context, input CalculateInput) (int64, error)
}

type Service struct {
	rateProvider RateProvider
}

func NewService(rateProvider RateProvider) *Service {
	return &Service{
		rateProvider: rateProvider,
	}
}

func (s *Service) Calculate(ctx context.Context, input CalculateInput) (CalculateResult, error) {
	if input.PurchaseAmountKop < 0 {
		return CalculateResult{}, domain.ErrInvalidAmount
	}
	rateBPS, err := s.rateProvider.GetRateBPS(ctx, input)
	if err != nil {
		return CalculateResult{}, fmt.Errorf("err get rate BPS: %w", err)
	}

	resultCash, err := domain.CalculateOptions(input.PurchaseAmountKop, 0, rateBPS)
	if err != nil {
		return CalculateResult{}, fmt.Errorf("err get options result: %w", err)
	}

	return CalculateResult{
		DiscountKop: resultCash.DiscountKop,
		PayableKop:  resultCash.PayableKop,
		CashbackKop: resultCash.CashbackKop,
	}, nil
}
