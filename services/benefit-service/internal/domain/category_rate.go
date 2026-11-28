package domain

func ApplyCategoryRate(rateBPS int64, categoryRateBPS int64, categorySelected bool) (int64, error) {
	if rateBPS < 0 || rateBPS > 10000 {
		return 0, ErrInvalidRate
	}
	if categoryRateBPS < 0 || categoryRateBPS > 10000 {
		return 0, ErrInvalidRate
	}
	if !categorySelected {
		return rateBPS, nil
	}
	return max(rateBPS, categoryRateBPS), nil
}
