package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

type UserProvider interface {
	GetProfile(ctx context.Context, userID string) (domain.UserProfile, error)
}

type OptionsProvider interface {
	GetOptions(ctx context.Context, input CalculateInput) ([]BestVariant, error)
}

type CashbackReserver interface {
	ReserveCashback(ctx context.Context, orderID, userID string, periodStart time.Time, requestedKopecks int64) (int64, error)
	GetReservByOrderID(ctx context.Context, orderID string) (domain.CashbackReservation, error)
}

type CashbackSetter interface {
	ConfirmCashback(ctx context.Context, orderID, userID string) (int64, error)
	CancelCashback(ctx context.Context, orderID, userID string) error
}

type Service struct {
	optionsProvider         OptionsProvider
	cashbackReserver        CashbackReserver
	cashabackSetter         CashbackSetter
	userProvider            UserProvider
	selectedCategoryRateBPS int64
	timeNow                 func() time.Time
}

func NewService(optionsProvider OptionsProvider, cashbackReserver CashbackReserver, cashbackSetter CashbackSetter, userProvider UserProvider, selectedCategoryRateBPS int64) *Service {
	return &Service{
		optionsProvider:         optionsProvider,
		cashbackReserver:        cashbackReserver,
		cashabackSetter:         cashbackSetter,
		userProvider:            userProvider,
		selectedCategoryRateBPS: selectedCategoryRateBPS,
		timeNow:                 time.Now,
	}
}

func (s *Service) Calculate(ctx context.Context, input CalculateInput) (domain.OptionResult, error) {
	status := domain.PersonalizationNotApplied
	if input.CategoryID == "" {
		return domain.OptionResult{}, domain.ErrNotCategory
	}
	if input.PurchaseAmountKop < 0 {
		return domain.OptionResult{}, domain.ErrInvalidAmount
	}
	if input.UserID == "" {
		return domain.OptionResult{}, domain.ErrNotUserID
	}
	profile, err := s.userProvider.GetProfile(ctx, input.UserID)
	if err := ctx.Err(); err != nil {
		return domain.OptionResult{}, fmt.Errorf("context error: %w", err)
	}
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrProfileNotFound):
			profile = domain.UserProfile{}

		case errors.Is(err, domain.ErrUserServiceUnavailable):
			profile = domain.UserProfile{}
			status = domain.PersonalizationUnavailable

		default:
			return domain.OptionResult{}, fmt.Errorf("get profile: %w", err)
		}
	}
	variants, err := s.optionsProvider.GetOptions(ctx, input)
	if err != nil {
		return domain.OptionResult{}, fmt.Errorf("error get list variant:%w", err)
	}

	if len(variants) == 0 {
		return domain.OptionResult{}, fmt.Errorf("variants list is empty")
	}

	var maxBenefit int64
	var bestResult *domain.OptionResult
	var isFirst bool = true

	categorySelected := slices.Contains(profile.SelectedCategoryIDs, input.CategoryID)
	if categorySelected {
		status = domain.PersonalizationApplied
	}

	for _, variant := range variants {
		rateBPS, err := domain.ApplyCategoryRate(variant.RateBPS, s.selectedCategoryRateBPS, categorySelected)
		if err != nil {
			return domain.OptionResult{}, fmt.Errorf("apply category rate: %w", err)
		}
		res, err := domain.CalculateOptions(input.PurchaseAmountKop, variant.DiscountKop, rateBPS)
		if err != nil {
			return domain.OptionResult{}, fmt.Errorf("calculate options: %w", err)
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
	bestResult.PersonalizationStatus = status
	return *bestResult, nil
}

func (s *Service) ConfirmCashback(ctx context.Context, orderID, userID string) (int64, error) {
	if err := validate(orderID, userID); err != nil {
		return 0, fmt.Errorf("error validate: %w", err)
	}
	amount, err := s.cashabackSetter.ConfirmCashback(ctx, orderID, userID)
	if err != nil {
		return 0, fmt.Errorf("err confirm cashback: %w", err)
	}
	return amount, nil
}

func (s *Service) CancelCashback(ctx context.Context, orderID, userID string) error {
	if err := validate(orderID, userID); err != nil {
		return fmt.Errorf("error validate: %w", err)
	}
	if err := s.cashabackSetter.CancelCashback(ctx, orderID, userID); err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	return nil
}
