package middleware

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Recovery(log *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if panicValue := recover(); panicValue != nil {
				log.WithFields(logrus.Fields{
					"method": info.FullMethod,
					"panic":  fmt.Sprint(panicValue),
					"stack":  string(debug.Stack()),
				}).Error("panic gRPC request")

				resp = nil
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
