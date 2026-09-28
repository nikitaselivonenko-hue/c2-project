package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Server — обёртка над http.Server с graceful shutdown.
type Server struct {
	httpServer *http.Server
}

// NewServer создаёт HTTP-сервер с маршрутизацией.
func NewServer(addr string, h *Handler) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/submit", h.Submit)
	mux.HandleFunc("/api/status", h.Status)
	mux.HandleFunc("/api/clients", h.Clients)
	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

// Run запускает сервер и блокируется до отмены контекста.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	}
}