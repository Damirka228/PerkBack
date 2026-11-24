package domain

import "time"

type OptionResult struct {
	DiscountKop int64
	PayableKop  int64
	CashbackKop int64
}

type RulePromotion struct {
	ID                   string
	ShopID               string
	CategoryID           string
	MinPurchaseAmountKop int64
	DiscountKop          int64
	RateBPS              int64
}

type CashbackReservation struct {
	UserID      string
	PeriodStart time.Time
	AmountKop   int64
	Status      string
}
