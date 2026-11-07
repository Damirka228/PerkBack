package domain

import (
	"fmt"
)

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

func CalculateOptions(payableKop int64, discountKop int64, rateBPS int64) (OptionResult, error) { //подсчет выгоды
	if payableKop < 0 {
		return OptionResult{}, fmt.Errorf("err invalid amount: %w", ErrInvalidAmount)
	}
	if discountKop < 0 || discountKop > payableKop {
		return OptionResult{}, fmt.Errorf("err invalid discount: %w", ErrInvalidDiscount)
	}
	payWithDisc := payableKop - discountKop
	cashback, err := CalculateCashback(payWithDisc, rateBPS)
	if err != nil {
		return OptionResult{}, fmt.Errorf("err to get cashback: %w", err)
	}
	return OptionResult{
		DiscountKop: discountKop,
		PayableKop:  payWithDisc,
		CashbackKop: cashback,
	}, nil
}
