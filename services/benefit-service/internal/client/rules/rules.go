package rules

import (
	"context"
	"fmt"
	"time"

	rulesv1 "github.com/Damirka228/PerkBack/gen/go/rules/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn           *grpc.ClientConn
	rpcClient      rulesv1.RulesServiceClient
	requestTimeout time.Duration
}

func NewClient(cfg config.RulesService) (*Client, error) {
	conn, err := grpc.NewClient(
		cfg.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("err to create grpc client:%w", err)
	}
	client := rulesv1.NewRulesServiceClient(conn)
	return &Client{
		conn:           conn,
		rpcClient:      client,
		requestTimeout: cfg.RequestTimeout,
	}, nil
}

func (c *Client) GetAllPromotionsClient(ctx context.Context) ([]domain.RulePromotion, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	resp, err := c.rpcClient.GetAllPromotions(reqCtx, &rulesv1.GetAllPromotionsRequest{})
	if err != nil {
		return nil, fmt.Errorf("err to get all promotions:%w", err)
	}

	var promotions []domain.RulePromotion
	for _, promo := range resp.GetPromotions() {
		promotions = append(promotions, domain.RulePromotion{
			ShopID:               promo.ShopId,
			CategoryID:           promo.CategoryId,
			MinPurchaseAmountKop: promo.MinPurchaseAmountKopecks,
			DiscountKop:          promo.DiscountKopecks,
			RateBPS:              promo.RateBps,
		})
	}

	return promotions, nil
}

func (c *Client) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("err to close grpc client:%w", err)
	}
	return nil
}
