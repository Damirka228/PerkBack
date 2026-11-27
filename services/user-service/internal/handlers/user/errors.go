package user

import (
	"context"
	"errors"

	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func getProfileError(err error) error {
	if errors.Is(err, domain.ErrUserIDEmpty) {
		return status.Error(codes.InvalidArgument, "user id is empty")
	}
	if errors.Is(err, domain.ErrProfileNotFound) {
		return status.Error(codes.NotFound, "profile not found")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	return status.Error(codes.Internal, "failed get profile")
}

func setSelectedCategoriesError(err error) error {
	if errors.Is(err, domain.ErrUserIDEmpty) {
		return status.Error(codes.InvalidArgument, "user id is empty")
	}
	if errors.Is(err, domain.ErrCategoryIDEmpty) {
		return status.Error(codes.InvalidArgument, "category id is empty")
	}
	if errors.Is(err, domain.ErrDuplicateCategory) {
		return status.Error(codes.InvalidArgument, "categories have duplicates")
	}
	if errors.Is(err, domain.ErrTooManyCategories) {
		return status.Error(codes.InvalidArgument, "at most 5 categories can be selected")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	return status.Error(codes.Internal, "failed save profile")
}
