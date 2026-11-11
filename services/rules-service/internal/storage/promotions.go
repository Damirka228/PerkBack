package storage

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
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
