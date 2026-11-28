package user

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	userv1 "github.com/Damirka228/PerkBack/gen/go/user/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Stub only the RPC used by the adapter.
type profileRPC struct {
	userv1.UserServiceClient
	getProfile func(context.Context, *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error)
}

func (r profileRPC) GetProfile(ctx context.Context, req *userv1.GetProfileRequest, _ ...grpc.CallOption) (*userv1.GetProfileResponse, error) {
	return r.getProfile(ctx, req)
}

func TestGetProfile(t *testing.T) {
	for _, categories := range [][]string{nil, {"groceries"}} {
		c := Client{requestTime: time.Second, rpcClient: profileRPC{
			getProfile: func(_ context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
				if got := req.GetUserId(); got != "user-1" {
					t.Fatalf("RPC user ID = %q, want user-1", got)
				}
				return &userv1.GetProfileResponse{UserId: req.GetUserId(), SelectedCategoryIds: categories}, nil
			},
		}}
		profile, err := c.GetProfile(context.Background(), "user-1")
		if err != nil || profile.UserID != "user-1" || !slices.Equal(profile.SelectedCategoryIDs, categories) {
			t.Fatalf("GetProfile() = %+v, %v; want user-1 with categories %v", profile, err, categories)
		}
	}
}

func TestGetProfileErrors(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	unexpected := status.Error(codes.Internal, "unexpected failure")

	for _, tt := range []struct {
		name    string
		parent  context.Context
		timeout time.Duration
		rpcErr  error
		want    error
	}{
		{"missing profile", context.Background(), time.Second, status.Error(codes.NotFound, "missing"), domain.ErrProfileNotFound},
		{"service unavailable", context.Background(), time.Second, status.Error(codes.Unavailable, "offline"), domain.ErrUserServiceUnavailable},
		{"client timeout", context.Background(), time.Millisecond, nil, domain.ErrUserServiceUnavailable},
		{"parent canceled", canceled, time.Second, nil, context.Canceled},
		{"parent deadline", expired, time.Second, nil, context.DeadlineExceeded},
		{"unexpected RPC error", context.Background(), time.Second, unexpected, unexpected},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := Client{requestTime: tt.timeout, rpcClient: profileRPC{
				getProfile: func(ctx context.Context, _ *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
					if tt.rpcErr != nil {
						return nil, tt.rpcErr
					}
					<-ctx.Done()
					return nil, status.FromContextError(ctx.Err()).Err()
				},
			}}
			if _, err := c.GetProfile(tt.parent, "user-1"); !errors.Is(err, tt.want) {
				t.Fatalf("GetProfile() error = %v, want %v", err, tt.want)
			}
		})
	}
}
