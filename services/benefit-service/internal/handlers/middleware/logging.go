package middleware

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func Logging(log *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		timeLogStart := time.Now()
		resp, err = handler(ctx, req)

		timeLogDur := time.Since(timeLogStart).Milliseconds()
		methodLog := info.FullMethod
		codeLog := status.Code(err).String()

		logging := log.WithFields(logrus.Fields{
			"method":      methodLog,
			"code":        codeLog,
			"duration_ms": timeLogDur,
		})

		if err != nil {
			logging := logging.WithError(err)
			logging.Error("gRPC request failed")
		} else {
			logging.Info("gRPC request success")
		}
		return resp, err
	}
}
