package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

func lockReservationForUser(ctx context.Context, orderId, userID string, tx pgx.Tx) (domain.CashbackReservation, error) {
	var reservation domain.CashbackReservation
	const sqlQuery = `
		SELECT user_id, period_start, amount_kopecks, status
		FROM cashback_reservations
		WHERE order_id = $1
		FOR UPDATE
	`
	row := tx.QueryRow(ctx, sqlQuery, orderId)
	err := row.Scan(&reservation.UserID, &reservation.PeriodStart, &reservation.AmountKop, &reservation.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CashbackReservation{}, domain.ErrReservationNotFound
	}
	if err != nil {
		return domain.CashbackReservation{}, fmt.Errorf("lock reservation: %w", err)
	}
	if reservation.UserID != userID {
		return domain.CashbackReservation{}, domain.ErrNotUserID
	}
	return reservation, nil
}

func updateReserveCashbackMonthLimit(ctx context.Context, amountKop int64, uID string, periodStart time.Time, tx pgx.Tx) error {
	const sqlQuery = `
		UPDATE cashback_monthly_limits
		SET reserved_kopecks = reserved_kopecks - $1,
			credited_kopecks = credited_kopecks + $1
		WHERE user_id = $2 AND period_start = $3
	`
	tag, err := tx.Exec(ctx, sqlQuery, amountKop, uID, periodStart)
	if err != nil {
		return fmt.Errorf("move reserved cashback to credited: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("monthly limit row not found")
	}
	return nil
}

func updateStatusReserveCashback(ctx context.Context, orderID string, tx pgx.Tx) error {
	const sqlQuery = `
		UPDATE cashback_reservations
		SET status = 'credited'
		WHERE order_id = $1 AND status = 'reserved'
	`
	tag, err := tx.Exec(ctx, sqlQuery, orderID)
	if err != nil {
		return fmt.Errorf("mark reservation as credited: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("reservation was not updated")
	}
	return nil
}

func updateReserveCancel(ctx context.Context, amountKop int64, uID string, periodStart time.Time, tx pgx.Tx) error {
	const sqlQuery = `
		UPDATE cashback_monthly_limits
		SET reserved_kopecks = reserved_kopecks - $1
		WHERE user_id = $2 AND period_start = $3
	`
	tag, err := tx.Exec(ctx, sqlQuery, amountKop, uID, periodStart)
	if err != nil {
		return fmt.Errorf("update row reserved kopecks: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("row limit not found")
	}

	return nil
}

func updateStatusCancel(ctx context.Context, orderID string, tx pgx.Tx) error {
	const sqlQuery = `
		UPDATE cashback_reservations
		SET status = 'canceled'
		WHERE order_id = $1 AND status = 'reserved'
	`
	tag, err := tx.Exec(ctx, sqlQuery, orderID)
	if err != nil {
		return fmt.Errorf("update status on canceled: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("row limit not found")
	}
	return nil
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
