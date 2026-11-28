package domain

import "time"

type PersonalizationStatus string

const (
	PersonalizationApplied     PersonalizationStatus = "APPLIED"
	PersonalizationNotApplied  PersonalizationStatus = "NOT_APPLIED"
	PersonalizationUnavailable PersonalizationStatus = "UNAVAILABLE"
)

type OptionResult struct {
	DiscountKop           int64
	PayableKop            int64
	CashbackKop           int64
	PersonalizationStatus PersonalizationStatus
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

type UserProfile struct {
	UserID              string
	SelectedCategoryIDs []string
}
