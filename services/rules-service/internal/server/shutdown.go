package server

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

func Shutdown(grpcServer *GRPCServer, timeout time.Duration, log *logrus.Logger) error {
	started := time.Now()
	log.WithField("timeout", timeout.String()).Info("starting graceful shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	grpcDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcDone)
	}()

	var shutdownErr error
	select {
	case <-grpcDone:
		log.Info("gRPC server stopped gracefully")
	case <-ctx.Done():
		log.Warn("shutdown deadline reached; forcing gRPC stop")
		grpcServer.Stop()
		shutdownErr = fmt.Errorf("gRPC shutdown: %w", ctx.Err())
	}

	log.WithField("duration", time.Since(started).String()).Info("gRPC shutdown finished")
	return shutdownErr
}
