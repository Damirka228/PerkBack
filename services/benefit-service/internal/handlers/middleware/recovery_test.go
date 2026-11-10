package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	benefitv1 "github.com/Damirka228/PerkBack/gen/go/benefit/v1"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/handlers/middleware"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoveryPassesHandlerResult(t *testing.T) {
	response := &benefitv1.CalculateBenefitResponse{
		PurchaseAmountKopecks: 100_000,
		PayableKopecks:        100_000,
		CashbackKopecks:       1_000,
	}
	handlerErr := errors.New("handler failed")

	tests := []struct {
		name     string
		response any
		err      error
	}{
		{
			name:     "successful_response",
			response: response,
		},
		{
			name: "handler_error",
			err:  handlerErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			log := logrus.New()
			log.SetOutput(&logs)
			log.SetFormatter(&logrus.JSONFormatter{})

			ctx := t.Context()
			request := &benefitv1.CalculateBenefitRequest{PurchaseAmountKopecks: 100_000}
			info := &grpc.UnaryServerInfo{
				FullMethod: benefitv1.BenefitService_CalculateBenefit_FullMethodName,
			}
			calls := 0
			handler := func(gotCtx context.Context, gotRequest any) (any, error) {
				calls++
				if gotCtx != ctx {
					t.Error("handler did not receive the original context")
				}
				if gotRequest != request {
					t.Error("handler did not receive the original request")
				}
				return tc.response, tc.err
			}

			got, err := middleware.Recovery(log)(ctx, request, info, handler)
			if got != tc.response {
				t.Errorf("response = %v, want the original response %v", got, tc.response)
			}
			if err != tc.err {
				t.Errorf("error = %v, want the original error %v", err, tc.err)
			}
			if calls != 1 {
				t.Errorf("handler calls = %d, want 1", calls)
			}
			if logs.Len() != 0 {
				t.Errorf("unexpected recovery log for a normal return: %s", logs.String())
			}
		})
	}
}

func TestRecoveryPanic(t *testing.T) {
	var logs bytes.Buffer
	log := logrus.New()
	log.SetOutput(&logs)
	log.SetFormatter(&logrus.JSONFormatter{})

	ctx := t.Context()
	request := &benefitv1.CalculateBenefitRequest{PurchaseAmountKopecks: 100_000}
	info := &grpc.UnaryServerInfo{
		FullMethod: benefitv1.BenefitService_CalculateBenefit_FullMethodName,
	}
	panicErr := errors.New("test panic in handler")
	calls := 0
	handler := func(ctx context.Context, req any) (any, error) {
		calls++
		panic(panicErr)
	}

	recovery := middleware.Recovery(log)
	response, err := recovery(ctx, request, info, handler)
	if response != nil {
		t.Errorf("response = %v, want nil after panic", response)
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("error = %v, want Internal", err)
	}
	if status.Convert(err).Message() != "internal server error" {
		t.Errorf("client error message = %q, want %q",
			status.Convert(err).Message(), "internal server error")
	}
	if calls != 1 {
		t.Errorf("handler calls = %d, want 1", calls)
	}

	var entry struct {
		Level  string
		Method string
		Panic  string
		Stack  string
	}
	decoder := json.NewDecoder(&logs)
	if err := decoder.Decode(&entry); err != nil {
		t.Fatalf("decode panic log: %v", err)
	}
	if entry.Level != "error" {
		t.Errorf("log level = %q, want error", entry.Level)
	}
	if entry.Method != info.FullMethod {
		t.Errorf("logged method = %q, want %q", entry.Method, info.FullMethod)
	}
	if entry.Panic != panicErr.Error() {
		t.Errorf("logged panic = %q, want %q", entry.Panic, panicErr.Error())
	}
	if !strings.Contains(entry.Stack, "TestRecoveryPanic") {
		t.Errorf("panic stack does not contain the handler call: %q", entry.Stack)
	}
	if err := decoder.Decode(&entry); err != io.EOF {
		t.Errorf("expected a single panic log, got trailing data or error: %v", err)
	}

	nextResponse := &benefitv1.CalculateBenefitResponse{PayableKopecks: 100_000}
	response, err = recovery(ctx, request, info, func(ctx context.Context, req any) (any, error) {
		return nextResponse, nil
	})
	if err != nil || response != nextResponse {
		t.Errorf("call after panic: response = %v, error = %v", response, err)
	}
}
