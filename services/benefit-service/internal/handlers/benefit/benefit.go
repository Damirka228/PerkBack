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

type CashbackSetter interface {
	ConfirmCashback(ctx context.Context, orderID, userID string) (int64, error)
	CancelCashback(ctx context.Context, orderID, userID string) error
}

type Handler struct {
	benefitv1.UnimplementedBenefitServiceServer
	calculator Calculator
	reserver   CashbackReserver
	setter     CashbackSetter
}

func NewHandler(calculator Calculator, reserver CashbackReserver, setter CashbackSetter) *Handler {
	return &Handler{
		calculator: calculator,
		reserver:   reserver,
		setter:     setter,
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

	var personalizationStatus benefitv1.PersonalizationStatus
	switch result.PersonalizationStatus {
	case domain.PersonalizationApplied:
		personalizationStatus = benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_APPLIED
	case domain.PersonalizationNotApplied:
		personalizationStatus = benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_NOT_APPLIED
	case domain.PersonalizationUnavailable:
		personalizationStatus = benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_UNAVAILABLE
	default:
		personalizationStatus = benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_UNSPECIFIED
	}

	return &benefitv1.CalculateBenefitResponse{
		PurchaseAmountKopecks: input.PurchaseAmountKop,
		DiscountKopecks:       result.DiscountKop,
		PayableKopecks:        result.PayableKop,
		CashbackKopecks:       result.CashbackKop,
		PersonalizationStatus: personalizationStatus,
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

func (h *Handler) ConfirmCashback(ctx context.Context, req *benefitv1.ConfirmCashbackRequest) (*benefitv1.ConfirmCashbackResponse, error) {
	amount, err := h.setter.ConfirmCashback(ctx, req.GetOrderId(), req.GetUserId())
	if err != nil {
		return nil, confirmError(err)
	}
	return &benefitv1.ConfirmCashbackResponse{
		CreditedCashbackKopecks: amount,
	}, nil
}

func (h *Handler) CancelCashback(ctx context.Context, req *benefitv1.CancelCashbackRequest) (*benefitv1.CancelCashbackResponse, error) {
	if err := h.setter.CancelCashback(ctx, req.GetOrderId(), req.GetUserId()); err != nil {
		return nil, cancelError(err)
	}
	return &benefitv1.CancelCashbackResponse{}, nil
}
