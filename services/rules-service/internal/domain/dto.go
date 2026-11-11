package domain

type Promotion struct {
	ID                   string
	ShopID               string
	CategoryID           string
	MinPurchaseAmountKop int64
	DiscountKop          int64
	RateBPS              int64
}
