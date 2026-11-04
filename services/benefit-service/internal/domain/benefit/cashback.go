package benefit

func CalculateCashback(payableKop int64, rateBPS int64) (int64, error) {
	if payableKop < 0 {
		return 0, ErrInvalidAmount
	}
	if rateBPS < 0 || rateBPS > 10000 {
		return 0, ErrInvalidRate
	}

	q := payableKop / 10000
	r := payableKop % 10000

	cashback := q*rateBPS + (r*rateBPS)/10000
	return cashback, nil
}
