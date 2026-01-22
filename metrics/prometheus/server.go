package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/p4gefau1t/trojan-go/config"
)

// Server represents the Prometheus metrics HTTP server
type Server struct {
	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a new Prometheus metrics server
func NewServer(ctx context.Context) (*Server, error) {
	cfg := config.FromContext(ctx, Name).(*Config)

	if !cfg.Metrics.Enabled {
		slog.Debug("prometheus metrics disabled")
		return nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)

	// Set server start time
	ServerStartTime.Set(float64(time.Now().Unix()))

	addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.Port)
	path := cfg.Metrics.Path
	if path == "" {
		path = "/metrics"
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s := &Server{
		server: server,
		ctx:    ctx,
		cancel: cancel,
	}

	return s, nil
}

// Start starts the Prometheus metrics server
func (s *Server) Start() error {
	if s == nil || s.server == nil {
		return nil
	}

	errChan := make(chan error, 1)
	go func() {
		slog.Info("prometheus metrics server started", "address", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-s.ctx.Done():
		return s.Close()
	}
}

// Close gracefully shuts down the metrics server
func (s *Server) Close() error {
	if s == nil || s.server == nil {
		return nil
	}

	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Info("shutting down prometheus metrics server")
	return s.server.Shutdown(ctx)
}

// RunMetricsServer runs the Prometheus metrics server in a goroutine
func RunMetricsServer(ctx context.Context) error {
	server, err := NewServer(ctx)
	if err != nil {
		return err
	}
	if server == nil {
		// Metrics disabled
		return nil
	}

	go func() {
		if err := server.Start(); err != nil {
			slog.Error("prometheus metrics server error", "error", err)
		}
	}()

	return nil
}
