package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

var (
	ErrOrderIDEmpty = errors.New("order id is empty")
	ErrUserIDEmpty  = errors.New("user id is empty")
)

func (s *Service) ReserveCashback(ctx context.Context, orderID string, input CalculateInput) (int64, error) {
	if err := validate(orderID, input.UserID); err != nil {
		return 0, fmt.Errorf("validate get reserv: %w", err)
	}
	reservCashback, err := s.cashbackReserver.GetReservByOrderID(ctx, orderID)
	switch {
	case err == nil:
		if input.UserID == reservCashback.UserID {
			return reservCashback.AmountKop, nil
		}
		return 0, domain.ErrNotUserID
	case errors.Is(err, domain.ErrReservationNotFound):
	default:
		return 0, fmt.Errorf("reserv cashback error: %w", err)
	}

	resultCashback, err := s.Calculate(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("calculate cashback: %w", err)
	}

	now := s.timeNow().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	amount, err := s.cashbackReserver.ReserveCashback(ctx, orderID, input.UserID, periodStart, resultCashback.CashbackKop)
	if err != nil {
		return 0, fmt.Errorf("reserve cashback: %w", err)
	}
	return amount, nil
}

func validate(orderID string, userID string) error {
	if strings.TrimSpace(orderID) == "" {
		return ErrOrderIDEmpty
	}
	if strings.TrimSpace(userID) == "" {
		return ErrUserIDEmpty
	}
	return nil
}
