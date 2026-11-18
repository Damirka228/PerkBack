package rules

import (
	"context"
	"errors"

	rulesv1 "github.com/Damirka228/PerkBack/gen/go/rules/v1"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Rules interface {
	GetAllPromotions(ctx context.Context) ([]domain.Promotion, error)
	CreatePromotion(ctx context.Context, promotion domain.Promotion) error
}

type Handler struct {
	rulesv1.UnimplementedRulesServiceServer
	rules Rules
}

func NewHandler(rules Rules) *Handler {
	return &Handler{
		rules: rules,
	}
}

func (h *Handler) GetAllPromotions(ctx context.Context, req *rulesv1.GetAllPromotionsRequest) (*rulesv1.GetAllPromotionsResponse, error) {
	promotions, err := h.rules.GetAllPromotions(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, status.Error(codes.Canceled, "request canceled")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Error(codes.DeadlineExceeded, "request deadline exceeded")
		}
		return nil, status.Error(codes.Internal, "failed to get promotions with grpc")
	}

	result := make([]*rulesv1.Promotion, 0, len(promotions))

	for _, promotion := range promotions {
		result = append(result, &rulesv1.Promotion{
			Id:                       promotion.ID,
			ShopId:                   promotion.ShopID,
			CategoryId:               promotion.CategoryID,
			MinPurchaseAmountKopecks: promotion.MinPurchaseAmountKop,
			DiscountKopecks:          promotion.DiscountKop,
			RateBps:                  promotion.RateBPS,
		})
	}
	return &rulesv1.GetAllPromotionsResponse{
		Promotions: result,
	}, nil
}

func (h *Handler) CreatePromotion(ctx context.Context, req *rulesv1.CreatePromotionRequest) (*rulesv1.CreatePromotionResponse, error) {
	promotion := domain.Promotion{
		ID:                   uuid.NewString(),
		ShopID:               req.GetShopId(),
		CategoryID:           req.GetCategoryId(),
		MinPurchaseAmountKop: req.GetMinPurchaseAmountKopecks(),
		DiscountKop:          req.GetDiscountKopecks(),
		RateBPS:              req.GetRateBps(),
	}

	if err := h.rules.CreatePromotion(ctx, promotion); err != nil {
		if errors.Is(err, domain.ErrInvalidPromotion) {
			return nil, status.Error(codes.InvalidArgument, "invalid promotions")
		}
		return nil, status.Error(codes.Internal, "could not create ptomotion")
	}

	return &rulesv1.CreatePromotionResponse{
		Id: promotion.ID,
	}, nil
}
