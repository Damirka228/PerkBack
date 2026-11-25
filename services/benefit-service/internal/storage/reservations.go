package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

var (
	ErrOrderConflict   = errors.New("order belongs to another user or period")
	ErrNegativeKopecks = errors.New("requested cashback must be non-negative")
	ErrUnknownStatus   = errors.New("unknow status is cashback reservation")
)

type monthlyLimit struct {
	limitKopecks    int64
	reservedKopecks int64
	creditedKopecks int64
}

type cashbackReservation struct {
	userID        string
	periodStart   time.Time
	amountKopecks int64
	status        string
}

func (s *Storage) GetReservByOrderID(ctx context.Context, orderID string) (domain.CashbackReservation, error) {
	var result domain.CashbackReservation
	const sqlQuery = `
		SELECT user_id, period_start, amount_kopecks, status
		FROM cashback_reservations
		WHERE order_id = $1
	`
	row := s.pool.QueryRow(ctx, sqlQuery, orderID)
	err := row.Scan(&result.UserID, &result.PeriodStart, &result.AmountKop, &result.Status)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.CashbackReservation{}, domain.ErrReservationNotFound
	case err != nil:
		return domain.CashbackReservation{}, fmt.Errorf("cashback reserv bd: %w", err)
	}
	return result, nil
}

func (s *Storage) ConfirmCashback(ctx context.Context, orderID, userID string) (int64, error) {
	var confirmedAmount int64
	err := s.transaction(ctx, s.log, func(tx pgx.Tx) error {
		reservation, err := lockReservationForUser(ctx, orderID, userID, tx)
		if err != nil {
			return err
		}
		switch {
		case reservation.Status == "credited":
			confirmedAmount = reservation.AmountKop
			return nil
		case reservation.Status == "canceled":
			return domain.ErrStatusCancel
		case reservation.Status == "reserved":
		default:
			return ErrUnknownStatus
		}
		if err := updateReserveCashbackMonthLimit(ctx, reservation.AmountKop, reservation.UserID, reservation.PeriodStart, tx); err != nil {
			return err
		}
		if err := updateStatusReserveCashback(ctx, orderID, tx); err != nil {
			return err
		}
		confirmedAmount = reservation.AmountKop
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("err transaction: %w", err)
	}
	return confirmedAmount, nil
}

func (s *Storage) CancelCashback(ctx context.Context, orderId, userId string) error {
	err := s.transaction(ctx, s.log, func(tx pgx.Tx) error {
		reservation, err := lockReservationForUser(ctx, orderId, userId, tx)
		if err != nil {
			return err
		}
		switch {
		case reservation.Status == "canceled":
			return nil
		case reservation.Status == "credited":
			return domain.ErrCanceledOrder
		case reservation.Status == "reserved":
		default:
			return ErrUnknownStatus
		}

		if err := updateReserveCancel(ctx, reservation.AmountKop, reservation.UserID, reservation.PeriodStart, tx); err != nil {
			return err
		}

		if err := updateStatusCancel(ctx, orderId, tx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("err transaction: %w", err)
	}
	return nil
}

func (s *Storage) ReserveCashback(ctx context.Context, orderID, userID string, periodStart time.Time, requestedKopecks int64) (int64, error) {
	if requestedKopecks < 0 {
		return 0, ErrNegativeKopecks
	}
	var amount int64
	err := s.transaction(ctx, s.log, func(tx pgx.Tx) error {
		limit, err := getOrCreateMonthlyLimitForUpdate(
			ctx, tx, userID, periodStart,
		)
		if err != nil {
			return err
		}

		existing, err := findReservation(ctx, tx, orderID)
		switch {
		case err == nil:
			samePeriod := existing.periodStart.Format(time.DateOnly) ==
				periodStart.Format(time.DateOnly)

			if existing.userID != userID || !samePeriod {
				return ErrOrderConflict
			}

			if existing.status == "canceled" {
				return domain.ErrReservationCanceled
			}

			amount = existing.amountKopecks
			return nil

		case errors.Is(err, pgx.ErrNoRows):
		default:
			return fmt.Errorf("get reservation: %w", err)
		}
		amount = calculateReservationAmount(limit, requestedKopecks)
		return saveReservation(
			ctx, tx, orderID, userID, periodStart, amount,
		)
	})

	if err != nil {
		return 0, err
	}
	return amount, nil
}
