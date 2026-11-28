package domain

import "errors"

var (
	ErrInvalidAmount          = errors.New("negative amount")
	ErrInvalidRate            = errors.New("the interest rate is out of range")
	ErrInvalidDiscount        = errors.New("the discount is out of range")
	ErrNotUserID              = errors.New("users didnt match")
	ErrReservationNotFound    = errors.New("reserv cashback not found")
	ErrReservationCanceled    = errors.New("cannot reserve cashback for a canceled order")
	ErrStatusCancel           = errors.New("cannot confirm a canceled reservation")
	ErrCanceledOrder          = errors.New("cannot cancel a credited reservation")
	ErrProfileNotFound        = errors.New("user profile not found")
	ErrUserServiceUnavailable = errors.New("user-service unavailable")
	ErrNotCategory            = errors.New("category is empty")
)
