package usecase

type CalculateInput struct {
	UserID            string
	MerchantID        string
	CategoryID        string
	PurchaseAmountKop int64
}

type BestVariant struct {
	DiscountKop int64
	RateBPS     int64
}
