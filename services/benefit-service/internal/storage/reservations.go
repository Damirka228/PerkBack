package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrOrderConflict   = errors.New("order belongs to another user or period")
	ErrNegativeKopecks = errors.New("requested cashback must be non-negative")
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

func getOrCreateMonthlyLimitForUpdate(ctx context.Context, tx pgx.Tx, userID string, periodStart time.Time) (monthlyLimit, error) {
	const insertQuery = `
		INSERT INTO cashback_monthly_limits (user_id, period_start)
		VALUES ($1, $2)
		ON CONFLICT (user_id, period_start) DO NOTHING;
	`
	if _, err := tx.Exec(ctx, insertQuery, userID, periodStart); err != nil {
		return monthlyLimit{}, fmt.Errorf("insert user id and period start: %w", err)
	}

	const selectQuery = `
		SELECT limit_kopecks, reserved_kopecks, credited_kopecks
		FROM cashback_monthly_limits
		WHERE user_id = $1 AND period_start = $2
		FOR UPDATE;
	`
	var limit monthlyLimit
	err := tx.QueryRow(ctx, selectQuery, userID, periodStart).Scan(
		&limit.limitKopecks, &limit.reservedKopecks, &limit.creditedKopecks,
	)
	if err != nil {
		return monthlyLimit{}, fmt.Errorf("err row scan: %w", err)
	}
	return limit, nil
}

func findReservation(ctx context.Context, tx pgx.Tx, orderID string) (cashbackReservation, error) {
	const query = `
		SELECT user_id, period_start, amount_kopecks, status
		FROM cashback_reservations
		WHERE order_id = $1
	`
	var reservation cashbackReservation
	err := tx.QueryRow(ctx, query, orderID).Scan(
		&reservation.userID, &reservation.periodStart, &reservation.amountKopecks, &reservation.status,
	)
	return reservation, err
}

func calculateReservationAmount(limit monthlyLimit, requestedKopecks int64) int64 {
	available := limit.limitKopecks - limit.reservedKopecks - limit.creditedKopecks
	return min(requestedKopecks, available)
}

func saveReservation(ctx context.Context, tx pgx.Tx, orderID, userID string, periodStart time.Time, amountKopecks int64) error {
	const updateQuery = `
		UPDATE cashback_monthly_limits
		SET reserved_kopecks = reserved_kopecks + $1
		WHERE user_id = $2 AND period_start = $3
	`
	if _, err := tx.Exec(ctx, updateQuery, amountKopecks, userID, periodStart); err != nil {
		return fmt.Errorf("sql query update limits: %w", err)
	}

	const insertQuery = `
		INSERT INTO cashback_reservations
		(order_id, user_id, period_start, amount_kopecks)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := tx.Exec(ctx, insertQuery, orderID, userID, periodStart, amountKopecks); err != nil {
		return fmt.Errorf("insert cashback reservation: %w", err)
	}
	return nil
}
