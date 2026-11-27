package user_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	userv1 "github.com/Damirka228/PerkBack/gen/go/user/v1"
	"github.com/Damirka228/PerkBack/services/user-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/user-service/internal/handlers/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type profileCall struct {
	ctx        context.Context
	userID     string
	categories []string
}

type fakeProfileService struct {
	getCalls []profileCall
	setCalls []profileCall
	profile  domain.Profile
	getErr   error
	setErr   error
}

func (f *fakeProfileService) GetProfile(ctx context.Context, userID string) (domain.Profile, error) {
	f.getCalls = append(f.getCalls, profileCall{ctx: ctx, userID: userID})
	return f.profile, f.getErr
}

func (f *fakeProfileService) SetSelectedCategories(ctx context.Context, userID string, categories []string) error {
	f.setCalls = append(f.setCalls, profileCall{ctx, userID, slices.Clone(categories)})
	return f.setErr
}

func TestGetProfileMapsResult(t *testing.T) {
	for _, tt := range []struct {
		name       string
		categories []string
	}{
		{"selected", []string{"transport", "food"}},
		{"empty", []string{}},
		{"nil", nil},
		{"five", []string{"food", "transport", "travel", "books", "sports"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			service := &fakeProfileService{
				profile: domain.Profile{UserID: "user-1", SelectedCategoryIDs: tt.categories},
			}
			handler := user.NewHandler(service)
			response, err := handler.GetProfile(ctx, &userv1.GetProfileRequest{UserId: "user-1"})
			if err != nil || response == nil {
				t.Fatalf("GetProfile() = (%v, %v), want profile response and nil error", response, err)
			}
			if response.GetUserId() != service.profile.UserID ||
				!slices.Equal(response.GetSelectedCategoryIds(), tt.categories) {
				t.Errorf("response = %+v, want user-1 and categories %v", response, tt.categories)
			}
			assertOnlyCall(t, service, "get", ctx, "user-1")
		})
	}
}

func TestSetSelectedCategoriesMapsRequest(t *testing.T) {
	for _, tt := range []struct {
		name       string
		categories []string
	}{
		{"selected", []string{"transport", "food"}},
		{"empty_clears_selection", []string{}},
		{"nil_clears_selection", nil},
		{"five", []string{"food", "transport", "travel", "books", "sports"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			service := &fakeProfileService{}
			handler := user.NewHandler(service)
			response, err := handler.SetSelectedCategories(ctx, &userv1.SetSelectedCategoriesRequest{
				UserId: "user-1", SelectedCategoryIds: tt.categories,
			})
			if err != nil || response == nil {
				t.Fatalf("SetSelectedCategories() = (%v, %v), want non-nil empty response and nil error", response, err)
			}
			assertOnlyCall(t, service, "set", ctx, "user-1")
			if !slices.Equal(service.setCalls[0].categories, tt.categories) {
				t.Errorf("usecase categories = %v, want %v", service.setCalls[0].categories, tt.categories)
			}
		})
	}
}

func TestProfileRPCErrors(t *testing.T) {
	tests := []struct {
		operation string
		name      string
		err       error
		code      codes.Code
		message   string
	}{
		{"get", "empty_user", domain.ErrUserIDEmpty, codes.InvalidArgument, "user id is empty"},
		{"get", "not_found", domain.ErrProfileNotFound, codes.NotFound, "profile not found"},
		{"get", "canceled", context.Canceled, codes.Canceled, "request canceled"},
		{"get", "deadline", context.DeadlineExceeded, codes.DeadlineExceeded, "request deadline exceeded"},
		{"get", "unexpected", errors.New("private database details"), codes.Internal, "failed get profile"},
		{"set", "empty_user", domain.ErrUserIDEmpty, codes.InvalidArgument, "user id is empty"},
		{"set", "empty_category", domain.ErrCategoryIDEmpty, codes.InvalidArgument, "category id is empty"},
		{"set", "duplicates", domain.ErrDuplicateCategory, codes.InvalidArgument, "categories have duplicates"},
		{"set", "too_many", domain.ErrTooManyCategories, codes.InvalidArgument, "at most 5 categories can be selected"},
		{"set", "canceled", context.Canceled, codes.Canceled, "request canceled"},
		{"set", "deadline", context.DeadlineExceeded, codes.DeadlineExceeded, "request deadline exceeded"},
		{"set", "unexpected", errors.New("private database details"), codes.Internal, "failed save profile"},
	}
	for _, tt := range tests {
		for _, wrapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/wrapped=%t", tt.operation, tt.name, wrapped), func(t *testing.T) {
				ctx := t.Context()
				serviceErr := tt.err
				if wrapped {
					serviceErr = fmt.Errorf("usecase operation: %w", serviceErr)
				}
				service := &fakeProfileService{
					// A partial result must not be returned alongside an error.
					profile: domain.Profile{UserID: "user-1", SelectedCategoryIDs: []string{"food"}},
					getErr:  serviceErr,
					setErr:  serviceErr,
				}
				handler := user.NewHandler(service)
				var err error
				if tt.operation == "get" {
					response, callErr := handler.GetProfile(ctx, &userv1.GetProfileRequest{UserId: "user-1"})
					if response != nil {
						t.Errorf("GetProfile response = %+v, want nil on error", response)
					}
					err = callErr
				} else {
					response, callErr := handler.SetSelectedCategories(ctx, &userv1.SetSelectedCategoriesRequest{
						UserId: "user-1", SelectedCategoryIds: []string{"food"},
					})
					if response != nil {
						t.Errorf("SetSelectedCategories response = %+v, want nil on error", response)
					}
					err = callErr
				}
				if status.Code(err) != tt.code || status.Convert(err).Message() != tt.message {
					t.Errorf("error = %v, want code %s and message %q", err, tt.code, tt.message)
				}
				assertOnlyCall(t, service, tt.operation, ctx, "user-1")
			})
		}
	}
}

func TestNilProfileRequestsUseGetters(t *testing.T) {
	for _, operation := range []string{"get", "set"} {
		t.Run(operation, func(t *testing.T) {
			ctx := t.Context()
			service := &fakeProfileService{getErr: domain.ErrUserIDEmpty, setErr: domain.ErrUserIDEmpty}
			handler := user.NewHandler(service)
			var err error
			if operation == "get" {
				response, callErr := handler.GetProfile(ctx, nil)
				if response != nil {
					t.Errorf("response = %+v, want nil", response)
				}
				err = callErr
			} else {
				response, callErr := handler.SetSelectedCategories(ctx, nil)
				if response != nil {
					t.Errorf("response = %+v, want nil", response)
				}
				err = callErr
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("nil request error = %v, want InvalidArgument", err)
			}
			assertOnlyCall(t, service, operation, ctx, "")
			if operation == "set" && len(service.setCalls[0].categories) != 0 {
				t.Error("nil request must pass an empty selection to the usecase")
			}
		})
	}
}

func assertOnlyCall(t *testing.T, service *fakeProfileService, operation string, ctx context.Context, userID string) {
	t.Helper()
	calls, other := service.getCalls, service.setCalls
	if operation == "set" {
		calls, other = service.setCalls, service.getCalls
	}
	if len(calls) != 1 || len(other) != 0 {
		t.Fatalf("%s calls = %d, other operation calls = %d; want 1 and 0", operation, len(calls), len(other))
	}
	if call := calls[0]; call.ctx != ctx || call.userID != userID {
		t.Errorf("usecase must receive original context and user ID %q, got ID %q", userID, call.userID)
	}
}
