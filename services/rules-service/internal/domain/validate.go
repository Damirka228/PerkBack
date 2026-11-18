package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPromotion = errors.New("invalid promotion")
)

func (p Promotion) Validate() error {
	if p.ShopID == "" {
		return fmt.Errorf("shop id is empty:%w", ErrInvalidPromotion)
	}
	if p.CategoryID == "" {
		return fmt.Errorf("category id is empty:%w", ErrInvalidPromotion)
	}
	if p.DiscountKop > p.MinPurchaseAmountKop {
		return fmt.Errorf("discount exceeds minimum purchase amount: %w", ErrInvalidPromotion)
	}
	if p.MinPurchaseAmountKop < 0 {
		return fmt.Errorf("purchase < 0:%w", ErrInvalidPromotion)
	}
	if p.DiscountKop < 0 {
		return fmt.Errorf("discount < 0:%w", ErrInvalidPromotion)
	}
	if p.RateBPS < 0 || p.RateBPS > 10000 {
		return fmt.Errorf("rate bps oot of range 0-10000:%w", ErrInvalidPromotion)
	}
	return nil
}
