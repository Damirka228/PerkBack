package server

import (
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
	if err := s.server.ListenAndServe(); err != nil {
		return fmt.Errorf("err start server: %w", err)
	}
	return nil
}
