package usecase

type CalculateInput struct {
	UserID            string
	ShopID            string
	CategoryID        string
	PurchaseAmountKop int64
}

type BestVariant struct {
	DiscountKop int64
	RateBPS     int64
}
