package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type HTTPServer struct {
	server *http.Server
}

func NewHTTPServer(addr string, handlers http.Handler, timeout time.Duration) *HTTPServer {
	server := &http.Server{
		Addr:              addr,
		Handler:           handlers,
		ReadHeaderTimeout: timeout,
	}
	return &HTTPServer{
		server: server,
	}
}

func (s *HTTPServer) Run() error {

	err := s.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	} else {
		return fmt.Errorf("err start server: %w", err)
	}
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("error to graceful shutdown http server: %w", err)
	}
	return nil
}

func (s *HTTPServer) Close() error {
	if err := s.server.Close(); err != nil {
		return fmt.Errorf("error to close http server: %w", err)
	}
	return nil
}
