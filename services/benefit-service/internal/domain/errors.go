package domain

import "errors"

var (
	ErrInvalidAmount       = errors.New("negative amount")
	ErrInvalidRate         = errors.New("the interest rate is out of range")
	ErrInvalidDiscount     = errors.New("the discount is out of range")
	ErrNotUserID           = errors.New("users didnt match")
	ErrReservationNotFound = errors.New("reserv cashback not found")
)
