package benefit

import benefitv1 "github.com/Damirka228/PerkBack/gen/go/benefit/v1"

type Handler struct {
	benefitv1.UnimplementedBenefitServiceServer
}

func NewHandler() *Handler {
	return &Handler{}
}

var _ benefitv1.BenefitServiceServer = (*Handler)(nil)
