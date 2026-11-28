package benefit

import (
	"context"
	"errors"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func calculateError(err error) error {
	if errors.Is(err, domain.ErrInvalidAmount) {
		return status.Error(codes.InvalidArgument, "purchase amount must be non-negative")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	if errors.Is(err, domain.ErrUserServiceUnavailable) {
		return status.Error(codes.Unavailable, "user-service unavalible")
	}
	return status.Error(codes.Internal, "failed to calculate benefit")

}

func reservError(err error) error {
	if errors.Is(err, usecase.ErrOrderIDEmpty) {
		return status.Error(codes.InvalidArgument, "order id is required")
	}
	if errors.Is(err, usecase.ErrUserIDEmpty) {
		return status.Error(codes.InvalidArgument, "user id is required")
	}
	if errors.Is(err, domain.ErrInvalidAmount) {
		return status.Error(codes.InvalidArgument, "purchase amount must be non-negative")
	}
	if errors.Is(err, domain.ErrNotUserID) {
		return status.Error(codes.PermissionDenied, "order belongs to another user")
	}
	if errors.Is(err, domain.ErrReservationCanceled) {
		return status.Error(codes.FailedPrecondition, "cannot reserve cashback for a canceled order")
	}
	if errors.Is(err, domain.ErrUserServiceUnavailable) {
		return status.Error(codes.Unavailable, "user-service unavalible")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	return status.Error(codes.Internal, "failed to reserve cashback")
}

func confirmError(err error) error {
	if errors.Is(err, usecase.ErrOrderIDEmpty) {
		return status.Error(codes.InvalidArgument, "order id is required")
	}
	if errors.Is(err, usecase.ErrUserIDEmpty) {
		return status.Error(codes.InvalidArgument, "user id is required")
	}
	if errors.Is(err, domain.ErrReservationNotFound) {
		return status.Error(codes.NotFound, "cashback reservation not found")
	}
	if errors.Is(err, domain.ErrNotUserID) {
		return status.Error(codes.PermissionDenied, "order belongs to another user")
	}
	if errors.Is(err, domain.ErrStatusCancel) {
		return status.Error(codes.FailedPrecondition, "cannot confirm a canceled reservation")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	return status.Error(codes.Internal, "failed to confirm cashback")
}

func cancelError(err error) error {
	if errors.Is(err, usecase.ErrOrderIDEmpty) {
		return status.Error(codes.InvalidArgument, "order id is required")
	}
	if errors.Is(err, usecase.ErrUserIDEmpty) {
		return status.Error(codes.InvalidArgument, "user id is required")
	}
	if errors.Is(err, domain.ErrReservationNotFound) {
		return status.Error(codes.NotFound, "cashback reservation not found")
	}
	if errors.Is(err, domain.ErrNotUserID) {
		return status.Error(codes.PermissionDenied, "order belongs to another user")
	}
	if errors.Is(err, domain.ErrCanceledOrder) {
		return status.Error(codes.FailedPrecondition, "cannot cancel a credited reservation")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	}
	return status.Error(codes.Internal, "failed to cancel cashback")
}
