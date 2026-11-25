package benefit_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	benefitv1 "github.com/Damirka228/PerkBack/gen/go/benefit/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/benefit"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type settlementCall struct {
	ctx     context.Context
	orderID string
	userID  string
}

type fakeCashbackSetter struct {
	confirmCalls []settlementCall
	cancelCalls  []settlementCall
	amount       int64
	confirmErr   error
	cancelErr    error
}

func (f *fakeCashbackSetter) ConfirmCashback(ctx context.Context, orderID, userID string) (int64, error) {
	f.confirmCalls = append(f.confirmCalls, settlementCall{ctx, orderID, userID})
	return f.amount, f.confirmErr
}

func (f *fakeCashbackSetter) CancelCashback(ctx context.Context, orderID, userID string) error {
	f.cancelCalls = append(f.cancelCalls, settlementCall{ctx, orderID, userID})
	return f.cancelErr
}

func TestConfirmCashbackMapsRequestAndAmount(t *testing.T) {
	for _, amount := range []int64{0, 5_000} {
		t.Run(fmt.Sprintf("amount_%d", amount), func(t *testing.T) {
			ctx := t.Context()
			setter := &fakeCashbackSetter{amount: amount}
			handler := benefit.NewHandler(nil, nil, setter)
			response, err := handler.ConfirmCashback(ctx, &benefitv1.ConfirmCashbackRequest{
				OrderId: "order-1", UserId: "user-1",
			})
			if err != nil || response == nil || response.GetCreditedCashbackKopecks() != amount {
				t.Fatalf("ConfirmCashback() = (%v, %v), want response with amount %d", response, err, amount)
			}
			assertSettlementCall(t, setter.confirmCalls, ctx, "order-1", "user-1")
			if len(setter.cancelCalls) != 0 {
				t.Error("confirmation called cancellation")
			}
		})
	}
}

func TestCancelCashbackMapsRequestAndReturnsEmptyResponse(t *testing.T) {
	ctx := t.Context()
	setter := &fakeCashbackSetter{}
	handler := benefit.NewHandler(nil, nil, setter)
	response, err := handler.CancelCashback(ctx, &benefitv1.CancelCashbackRequest{
		OrderId: "order-1", UserId: "user-1",
	})
	if err != nil || response == nil {
		t.Fatalf("CancelCashback() = (%v, %v), want non-nil empty response and nil error", response, err)
	}
	assertSettlementCall(t, setter.cancelCalls, ctx, "order-1", "user-1")
	if len(setter.confirmCalls) != 0 {
		t.Error("cancellation called confirmation")
	}
}

func TestSettlementErrors(t *testing.T) {
	operations := []struct {
		name         string
		stateErr     error
		stateMessage string
	}{
		{"confirm", domain.ErrStatusCancel, "cannot confirm a canceled reservation"},
		{"cancel", domain.ErrCanceledOrder, "cannot cancel a credited reservation"},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			tests := []struct {
				name    string
				err     error
				code    codes.Code
				message string
			}{
				{"empty_order", usecase.ErrOrderIDEmpty, codes.InvalidArgument, "order id is required"},
				{"empty_user", usecase.ErrUserIDEmpty, codes.InvalidArgument, "user id is required"},
				{"missing_reservation", domain.ErrReservationNotFound, codes.NotFound, "cashback reservation not found"},
				{"foreign_order", domain.ErrNotUserID, codes.PermissionDenied, "order belongs to another user"},
				{"invalid_state", operation.stateErr, codes.FailedPrecondition, operation.stateMessage},
				{"canceled", context.Canceled, codes.Canceled, "request canceled"},
				{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded, "request deadline exceeded"},
				{"unexpected", errors.New("private database details"), codes.Internal, "failed to " + operation.name + " cashback"},
			}
			for _, tt := range tests {
				for _, wrapped := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/wrapped=%t", tt.name, wrapped), func(t *testing.T) {
						ctx := t.Context()
						usecaseErr := tt.err
						if wrapped {
							usecaseErr = fmt.Errorf("settlement operation: %w", usecaseErr)
						}
						setter := &fakeCashbackSetter{
							amount: 5_000, confirmErr: usecaseErr, cancelErr: usecaseErr,
						}
						handler := benefit.NewHandler(nil, nil, setter)
						var err error
						if operation.name == "confirm" {
							response, callErr := handler.ConfirmCashback(ctx, &benefitv1.ConfirmCashbackRequest{
								OrderId: "order-1", UserId: "user-1",
							})
							if response != nil {
								t.Errorf("confirmation response = %+v, want nil on error", response)
							}
							err = callErr
							assertSettlementCall(t, setter.confirmCalls, ctx, "order-1", "user-1")
							if len(setter.cancelCalls) != 0 {
								t.Error("confirmation called cancellation")
							}
						} else {
							response, callErr := handler.CancelCashback(ctx, &benefitv1.CancelCashbackRequest{
								OrderId: "order-1", UserId: "user-1",
							})
							if response != nil {
								t.Errorf("cancellation response = %+v, want nil on error", response)
							}
							err = callErr
							assertSettlementCall(t, setter.cancelCalls, ctx, "order-1", "user-1")
							if len(setter.confirmCalls) != 0 {
								t.Error("cancellation called confirmation")
							}
						}
						if status.Code(err) != tt.code || status.Convert(err).Message() != tt.message {
							t.Errorf("error = %v, want code %s and message %q", err, tt.code, tt.message)
						}
					})
				}
			}
		})
	}
}

func TestSettlementNilRequestsUseGetters(t *testing.T) {
	ctx := t.Context()
	setter := &fakeCashbackSetter{
		confirmErr: usecase.ErrOrderIDEmpty, cancelErr: usecase.ErrOrderIDEmpty,
	}
	handler := benefit.NewHandler(nil, nil, setter)

	confirmed, err := handler.ConfirmCashback(ctx, nil)
	if confirmed != nil || status.Code(err) != codes.InvalidArgument {
		t.Errorf("nil confirmation request = (%v, %v), want nil and InvalidArgument", confirmed, err)
	}
	assertSettlementCall(t, setter.confirmCalls, ctx, "", "")

	canceled, err := handler.CancelCashback(ctx, nil)
	if canceled != nil || status.Code(err) != codes.InvalidArgument {
		t.Errorf("nil cancellation request = (%v, %v), want nil and InvalidArgument", canceled, err)
	}
	assertSettlementCall(t, setter.cancelCalls, ctx, "", "")
}

func assertSettlementCall(t *testing.T, calls []settlementCall, ctx context.Context, orderID, userID string) {
	t.Helper()
	if len(calls) != 1 {
		t.Fatalf("usecase calls = %d, want 1", len(calls))
	}
	if call := calls[0]; call.ctx != ctx || call.orderID != orderID || call.userID != userID {
		t.Errorf("usecase must receive original context, order=%q, user=%q; got order=%q, user=%q",
			orderID, userID, call.orderID, call.userID)
	}
}
