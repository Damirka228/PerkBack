package domain

func (r RulePromotion) MatchesRule(shopID string, categoryID string, purchaseAmountKop int64) bool {
	if r.ShopID == shopID && r.CategoryID == categoryID && purchaseAmountKop >= r.MinPurchaseAmountKop {
		return true
	}
	return false
}
