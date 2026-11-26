package domain

import "errors"

var (
	ErrUserIDEmpty       = errors.New("user id is empty")
	ErrCategoryIDEmpty   = errors.New("category id is empty")
	ErrDuplicateCategory = errors.New("selected categories contain a duplicate")
	ErrTooManyCategories = errors.New("at most 5 categories can be selected")
	ErrProfileNotFound   = errors.New("profile not found")
)
