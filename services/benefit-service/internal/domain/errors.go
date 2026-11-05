package domain

import "errors"

var (
	ErrInvalidAmount = errors.New("negative amount")
	ErrInvalidRate   = errors.New("the interest rate is out of range")
)
