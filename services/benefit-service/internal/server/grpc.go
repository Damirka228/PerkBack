package server

import (
	"fmt"
	"net"

	benefitv1 "github.com/Damirka228/PerkBack/gen/go/benefit/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/middleware"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	addr   string
	server *grpc.Server
}

func NewGRPCServer(addr string, handler benefitv1.BenefitServiceServer, log *logrus.Logger) *GRPCServer {
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.Logging(log),
			middleware.Recovery(log),
		),
	)
	benefitv1.RegisterBenefitServiceServer(server, handler)
	return &GRPCServer{
		addr:   addr,
		server: server,
	}
}

func (s *GRPCServer) Run() error {
	listen, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("error to listen: %w", err)
	}
	if err := s.server.Serve(listen); err != nil {
		return fmt.Errorf("error to serve: %w", err)
	}
	return nil
}

func (s *GRPCServer) GracefulStop() {
	s.server.GracefulStop()
}

func (s *GRPCServer) Stop() {
	s.server.Stop()
}
