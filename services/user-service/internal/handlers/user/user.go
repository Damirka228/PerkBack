package user

import (
	"context"

	userv1 "github.com/Damirka228/PerkBack/gen/go/user/v1"
	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
)

type ProfileService interface {
	SetSelectedCategories(ctx context.Context, userID string, categoryIDs []string) error
	GetProfile(ctx context.Context, userID string) (domain.Profile, error)
}

type Handler struct {
	userv1.UnimplementedUserServiceServer
	profileService ProfileService
}

func NewHandler(profileService ProfileService) *Handler {
	return &Handler{profileService: profileService}
}

func (h *Handler) GetProfile(ctx context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	uID := req.GetUserId()
	profile, err := h.profileService.GetProfile(ctx, uID)
	if err != nil {
		return nil, getProfileError(err)
	}
	return &userv1.GetProfileResponse{
		UserId:              profile.UserID,
		SelectedCategoryIds: profile.SelectedCategoryIDs,
	}, nil
}

func (h *Handler) SetSelectedCategories(ctx context.Context, req *userv1.SetSelectedCategoriesRequest) (*userv1.SetSelectedCategoriesResponse, error) {
	uID := req.GetUserId()
	categories := req.GetSelectedCategoryIds()
	err := h.profileService.SetSelectedCategories(ctx, uID, categories)
	if err != nil {
		return nil, setSelectedCategoriesError(err)
	}
	return &userv1.SetSelectedCategoriesResponse{}, nil
}
