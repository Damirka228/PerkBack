package benefit

import (
	"context"
	"errors"

	benefitv1 "github.com/Damirka228/PerkBack/gen/go/benefit/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Calculator interface {
	Calculate(ctx context.Context, input usecase.CalculateInput) (domain.OptionResult, error)
}

type Handler struct {
	benefitv1.UnimplementedBenefitServiceServer
	calculator Calculator
}

func NewHandler(calculator Calculator) *Handler {
	return &Handler{
		calculator: calculator,
	}
}

func (h *Handler) CalculateBenefit(ctx context.Context, req *benefitv1.CalculateBenefitRequest) (*benefitv1.CalculateBenefitResponse, error) {
	input := usecase.CalculateInput{
		UserID:            req.GetUserId(),
		MerchantID:        req.GetMerchantId(),
		CategoryID:        req.GetCategoryId(),
		PurchaseAmountKop: req.GetPurchaseAmountKopecks(),
	}

	result, err := h.calculator.Calculate(ctx, input)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidAmount) {
			return nil, status.Error(codes.InvalidArgument, "purchase amount must be non-negative")
		}
		if errors.Is(err, context.Canceled) {
			return nil, status.Error(codes.Canceled, "request canceled")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, "request deadline exceeded")
		}
		return nil, status.Error(codes.Internal, "failed to calculate benefit")
	}
	return &benefitv1.CalculateBenefitResponse{
		DiscountKopecks: result.DiscountKop,
		PayableKopecks:  result.PayableKop,
		CashbackKopecks: result.CashbackKop,
	}, nil
}
