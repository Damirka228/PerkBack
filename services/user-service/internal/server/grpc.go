package server

import (
	"fmt"
	"net"

	userv1 "github.com/Damirka228/PerkBack/gen/go/user/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	addr   string
	server *grpc.Server
}

func NewGRPCServer(addr string, handler userv1.UserServiceServer, log *logrus.Logger) *GRPCServer {
	server := grpc.NewServer()
	userv1.RegisterUserServiceServer(server, handler)

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
