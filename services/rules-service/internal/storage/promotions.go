package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"github.com/google/uuid"
)

const query = `
		SELECT id::text, shop_id, category_id, min_purchase_amount_kopecks, discount_kopecks, rate_bps
		FROM promotions
		ORDER BY id ASC;
	`

func (s *Storage) GetAllPromotions(ctx context.Context) ([]domain.Promotion, error) {

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query promotions: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Promotion, 0)
	for rows.Next() {
		var promotion domain.Promotion
		if err := rows.Scan(
			&promotion.ID,
			&promotion.ShopID,
			&promotion.CategoryID,
			&promotion.MinPurchaseAmountKop,
			&promotion.DiscountKop,
			&promotion.RateBPS,
		); err != nil {
			return nil, fmt.Errorf("scan promotion: %w", err)
		}
		result = append(result, promotion)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotions: %w", err)
	}

	return result, nil
}

func (s *Storage) CreatePromotion(ctx context.Context, promotion domain.Promotion) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("err to start transaction:%w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
		return
	}()

	sqlPromotions := `
		INSERT INTO promotions
		(id, shop_id, category_id, min_purchase_amount_kopecks, discount_kopecks, rate_bps)
		VALUES ($1, $2, $3, $4, $5, $6);
	`
	_, err = tx.Exec(
		ctx,
		sqlPromotions,
		promotion.ID,
		promotion.ShopID,
		promotion.CategoryID,
		promotion.MinPurchaseAmountKop,
		promotion.DiscountKop,
		promotion.RateBPS,
	)
	if err != nil {
		return fmt.Errorf("err sql promotions:%w", err)
	}

	payload, err := json.Marshal(promotion)
	if err != nil {
		return fmt.Errorf("err json format marshall:%w", err)
	}
	sqlPromotionsOutbox := `
		INSERT INTO promotions_outbox
		(id, event_type, payload)
		VALUES ($1, $2, $3);
	`
	_, err = tx.Exec(
		ctx,
		sqlPromotionsOutbox,
		uuid.NewString(),
		"promotion.created",
		payload,
	)
	if err != nil {
		return fmt.Errorf("err sql promotions outbox:%w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit promotion transaction: %w", err)
	}
	return nil
}
