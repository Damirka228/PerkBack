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

type reserverFunc func(context.Context, string, usecase.CalculateInput) (int64, error)

func (f reserverFunc) ReserveCashback(ctx context.Context, orderID string, input usecase.CalculateInput) (int64, error) {
	return f(ctx, orderID, input)
}

func TestReserveCashbackErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		code    codes.Code
		message string
	}{
		{"empty_order", usecase.ErrOrderIDEmpty, codes.InvalidArgument, "order id is required"},
		{"empty_user", usecase.ErrUserIDEmpty, codes.InvalidArgument, "user id is required"},
		{"negative_purchase", domain.ErrInvalidAmount, codes.InvalidArgument, "purchase amount must be non-negative"},
		{"foreign_order", domain.ErrNotUserID, codes.PermissionDenied, "order belongs to another user"},
		{"canceled", context.Canceled, codes.Canceled, "request canceled"},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded, "request deadline exceeded"},
		{"unexpected", errors.New("private database details"), codes.Internal, "failed to reserve cashback"},
	}
	for _, tt := range tests {
		for _, wrapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wrapped=%t", tt.name, wrapped), func(t *testing.T) {
				calls := 0
				handler := benefit.NewHandler(nil, reserverFunc(func(context.Context, string, usecase.CalculateInput) (int64, error) {
					calls++
					err := tt.err
					if wrapped {
						err = fmt.Errorf("usecase operation: %w", err)
					}
					return 2_000, err
				}))
				response, err := handler.ReserveCashback(t.Context(), &benefitv1.ReserveCashbackRequest{
					OrderId: "order-1", UserId: "user-1", PurchaseAmountKopecks: 100_000,
				})
				if response != nil {
					t.Errorf("response = %+v, want nil on error", response)
				}
				if status.Code(err) != tt.code || status.Convert(err).Message() != tt.message {
					t.Errorf("error = %v, want code %s and message %q", err, tt.code, tt.message)
				}
				if calls != 1 {
					t.Errorf("usecase calls = %d, want 1", calls)
				}
			})
		}
	}
}

func TestReserveCashbackMapsRequestAndActualAmount(t *testing.T) {
	for _, amount := range []int64{0, 2_000} {
		t.Run(fmt.Sprintf("amount_%d", amount), func(t *testing.T) {
			ctx := t.Context()
			calls := 0
			handler := benefit.NewHandler(nil, reserverFunc(func(gotCtx context.Context, orderID string, input usecase.CalculateInput) (int64, error) {
				calls++
				want := usecase.CalculateInput{
					UserID: "user-1", ShopID: "shop-42", CategoryID: "groceries", PurchaseAmountKop: 100_000,
				}
				if gotCtx != ctx || orderID != "order-1" || input != want {
					t.Errorf("usecase must receive original context, order-1 and %+v; got order=%q, input=%+v",
						want, orderID, input)
				}
				return amount, nil
			}))
			response, err := handler.ReserveCashback(ctx, &benefitv1.ReserveCashbackRequest{
				OrderId: "order-1", UserId: "user-1", ShopId: "shop-42",
				CategoryId: "groceries", PurchaseAmountKopecks: 100_000,
			})
			if err != nil || response == nil || response.GetReservedCashbackKopecks() != amount {
				t.Fatalf("ReserveCashback() = (%v, %v), want response with amount %d", response, err, amount)
			}
			if calls != 1 {
				t.Errorf("usecase calls = %d, want 1", calls)
			}
		})
	}
}

func TestReserveCashbackNilRequestUsesGetters(t *testing.T) {
	calls := 0
	handler := benefit.NewHandler(nil, reserverFunc(func(_ context.Context, orderID string, input usecase.CalculateInput) (int64, error) {
		calls++
		if orderID != "" || input != (usecase.CalculateInput{}) {
			t.Errorf("nil request produced order=%q and input=%+v, want zero values", orderID, input)
		}
		return 0, usecase.ErrOrderIDEmpty
	}))
	response, err := handler.ReserveCashback(t.Context(), nil)
	if response != nil || status.Code(err) != codes.InvalidArgument || calls != 1 {
		t.Fatalf("nil request: response=%v, error=%v, calls=%d; want nil, InvalidArgument, 1", response, err, calls)
	}
}
