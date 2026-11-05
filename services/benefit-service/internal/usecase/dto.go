package usecase

type CalculateInput struct {
	UserID            string
	MerchantID        string
	CategoryID        string
	PurchaseAmountKop int64
}

type CalculateResult struct {
	DiscountKop int64
	PayableKop  int64
	CashbackKop int64
}
