package domain

type OptionResult struct {
	DiscountKop int64
	PayableKop  int64
	CashbackKop int64
}

type RulePromotion struct {
	ShopID               string
	CategoryID           string
	MinPurchaseAmountKop int64
	DiscountKop          int64
	RateBPS              int64
}
