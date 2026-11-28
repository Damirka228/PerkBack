package benefit_test

import (
	"context"
	"testing"

	benefitv1 "github.com/Damirka228/PerkBack/gen/go/benefit/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/benefit"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/usecase"
)

type calculatorFunc func(context.Context, usecase.CalculateInput) (domain.OptionResult, error)

func (f calculatorFunc) Calculate(ctx context.Context, input usecase.CalculateInput) (domain.OptionResult, error) {
	return f(ctx, input)
}

func TestCalculateBenefitMapsPersonalizationStatus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status domain.PersonalizationStatus
		want   benefitv1.PersonalizationStatus
	}{
		{"applied", domain.PersonalizationApplied, benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_APPLIED},
		{"not_applied", domain.PersonalizationNotApplied, benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_NOT_APPLIED},
		{"unavailable", domain.PersonalizationUnavailable, benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_UNAVAILABLE},
		{"unknown", "unknown", benefitv1.PersonalizationStatus_PERSONALIZATION_STATUS_UNSPECIFIED},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := benefit.NewHandler(calculatorFunc(func(context.Context, usecase.CalculateInput) (domain.OptionResult, error) {
				return domain.OptionResult{PersonalizationStatus: tt.status}, nil
			}), nil, nil)

			got, err := handler.CalculateBenefit(t.Context(), &benefitv1.CalculateBenefitRequest{})
			if err != nil || got == nil || got.GetPersonalizationStatus() != tt.want {
				t.Fatalf("CalculateBenefit() = (%v, %v), want response with status %v", got, err, tt.want)
			}
		})
	}
}
