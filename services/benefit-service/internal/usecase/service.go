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
	RateBPS, err := s.rateProvider.GetRateBPS(ctx, input)
	if err != nil {
		return CalculateResult{}, fmt.Errorf("err get rate BPS: %w", err)
	}

	cashback, err := domain.CalculateCashback(input.PurchaseAmountKop, RateBPS)
	if err != nil {
		return CalculateResult{}, fmt.Errorf("err calculate cashback: %w", err)
	}
	return CalculateResult{
		DiscountKop: 0,
		PayableKop:  input.PurchaseAmountKop,
		CashbackKop: cashback,
	}, nil
}
