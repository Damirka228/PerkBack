package user

import (
	"context"
	"fmt"
	"time"

	userv1 "github.com/Damirka228/PerkBack/gen/go/user/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Client struct {
	conn        *grpc.ClientConn
	rpcClient   userv1.UserServiceClient
	requestTime time.Duration
}

func NewClient(cfg config.UserService) (*Client, error) {
	conn, err := grpc.NewClient(
		cfg.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create grpc client: %w", err)
	}

	client := userv1.NewUserServiceClient(conn)
	return &Client{
		conn:        conn,
		rpcClient:   client,
		requestTime: cfg.RequestTimeout,
	}, nil
}

func (c *Client) GetProfile(ctx context.Context, userID string) (domain.UserProfile, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.requestTime)
	defer cancel()

	resp, err := c.rpcClient.GetProfile(reqCtx, &userv1.GetProfileRequest{UserId: userID})
	if err := ctx.Err(); err != nil {
		return domain.UserProfile{}, err
	}
	if err != nil {
		code := status.Code(err)
		switch {
		case code == codes.NotFound:
			return domain.UserProfile{}, domain.ErrProfileNotFound
		case code == codes.Unavailable || reqCtx.Err() == context.DeadlineExceeded:
			return domain.UserProfile{}, domain.ErrUserServiceUnavailable
		}
		return domain.UserProfile{}, fmt.Errorf("get profile: %w", err)
	}
	return domain.UserProfile{
		UserID:              resp.UserId,
		SelectedCategoryIDs: resp.SelectedCategoryIds,
	}, nil
}

func (c *Client) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("close grpc client: %w", err)
	}
	return nil
}
