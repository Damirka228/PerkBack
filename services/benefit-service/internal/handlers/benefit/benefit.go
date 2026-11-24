package benefit

import (
	"context"

	benefitv1 "github.com/Damirka228/PerkBack/gen/go/benefit/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
)

type Calculator interface {
	Calculate(ctx context.Context, input usecase.CalculateInput) (domain.OptionResult, error)
}

type CashbackReserver interface {
	ReserveCashback(ctx context.Context, orderID string, input usecase.CalculateInput) (int64, error)
}

type Handler struct {
	benefitv1.UnimplementedBenefitServiceServer
	calculator Calculator
	reserver   CashbackReserver
}

func NewHandler(calculator Calculator, reserver CashbackReserver) *Handler {
	return &Handler{
		calculator: calculator,
		reserver:   reserver,
	}
}

func (h *Handler) CalculateBenefit(ctx context.Context, req *benefitv1.CalculateBenefitRequest) (*benefitv1.CalculateBenefitResponse, error) {
	input := usecase.CalculateInput{
		UserID:            req.GetUserId(),
		ShopID:            req.GetShopId(),
		CategoryID:        req.GetCategoryId(),
		PurchaseAmountKop: req.GetPurchaseAmountKopecks(),
	}

	result, err := h.calculator.Calculate(ctx, input)
	if err != nil {
		return nil, calculateError(err)
	}
	return &benefitv1.CalculateBenefitResponse{
		PurchaseAmountKopecks: input.PurchaseAmountKop,
		DiscountKopecks:       result.DiscountKop,
		PayableKopecks:        result.PayableKop,
		CashbackKopecks:       result.CashbackKop,
	}, nil
}

func (h *Handler) ReserveCashback(ctx context.Context, req *benefitv1.ReserveCashbackRequest) (*benefitv1.ReserveCashbackResponse, error) {
	input := usecase.CalculateInput{
		UserID:            req.GetUserId(),
		ShopID:            req.GetShopId(),
		CategoryID:        req.GetCategoryId(),
		PurchaseAmountKop: req.GetPurchaseAmountKopecks(),
	}
	amount, err := h.reserver.ReserveCashback(ctx, req.GetOrderId(), input)
	if err != nil {
		return nil, reservError(err)
	}
	return &benefitv1.ReserveCashbackResponse{
		ReservedCashbackKopecks: amount,
	}, nil
}
