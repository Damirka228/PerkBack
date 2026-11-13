package rules

import (
	"context"
	"errors"

	rulesv1 "github.com/Damirka228/PerkBack/gen/go/rules/v1"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Rules interface {
	GetAllPromotions(ctx context.Context) ([]domain.Promotion, error)
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
