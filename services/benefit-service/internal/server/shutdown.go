package server

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

func Shutdown(httpServer *HTTPServer, grpcServer *GRPCServer, timeout time.Duration, log *logrus.Logger) error {
	started := time.Now()
	log.WithField("timeout", timeout.String()).Info("starting graceful shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	grpcDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcDone)
	}()

	httpErr := httpServer.Shutdown(ctx)
	var closeErr error
	if httpErr != nil {
		log.WithField("server", "http").Warn("graceful shutdown failed; forcing connection close")
		closeErr = httpServer.Close()
	} else {
		log.WithField("server", "http").Info("server stopped gracefully")
	}

	var timeoutErr error
	select {
	case <-grpcDone:
	default:
		select {
		case <-grpcDone:
		case <-ctx.Done():
			log.WithField("server", "grpc").Warn("shutdown deadline reached; forcing stop")
			grpcServer.Stop()
			timeoutErr = fmt.Errorf("gRPC shutdown: %w", ctx.Err())
		}
	}
	if timeoutErr == nil {
		log.WithField("server", "grpc").Info("server stopped gracefully")
	}

	var resultErr error
	for _, err := range []error{httpErr, closeErr, timeoutErr} {
		if err == nil {
			continue
		}
		if resultErr == nil {
			resultErr = err
		} else {
			log.WithError(err).Error("additional shutdown error")
		}
	}

	entry := log.WithField("duration", time.Since(started).String())
	if resultErr != nil {
		entry.Warn("server shutdown finished with errors")
	} else {
		entry.Info("graceful shutdown completed")
	}
	return resultErr
}
