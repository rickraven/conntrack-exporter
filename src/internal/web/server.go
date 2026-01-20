package web

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"conntrack-exporter/internal/logging"
)

// Server exposes Prometheus metrics via HTTP.
type Server struct {
	Logger *logging.Logger

	Registry       *prometheus.Registry
	TelemetryPath  string
	ListenAddrs    []string
	MaxRequests    int
	DisableExpMetrics bool
}

// Start launches HTTP servers for all configured listen addresses.
// It blocks until ctx is cancelled, then attempts a graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	if s.Registry == nil {
		s.Registry = prometheus.NewRegistry()
	}
	if s.TelemetryPath == "" {
		s.TelemetryPath = "/metrics"
	}

	handlerOpts := promhttp.HandlerOpts{}
	if s.MaxRequests > 0 {
		handlerOpts.MaxRequestsInFlight = s.MaxRequests
	}

	baseHandler := promhttp.HandlerFor(s.Registry, handlerOpts)
	var metricsHandler http.Handler = baseHandler

	// promhttp_ metrics are only registered if we wrap with InstrumentMetricHandler.
	if !s.DisableExpMetrics {
		metricsHandler = promhttp.InstrumentMetricHandler(s.Registry, baseHandler)
	}

	mux := http.NewServeMux()
	mux.Handle(s.TelemetryPath, metricsHandler)

	errCh := make(chan error, len(s.ListenAddrs))
	servers := make([]*http.Server, 0, len(s.ListenAddrs))

	for _, addr := range s.ListenAddrs {
		srv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		servers = append(servers, srv)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}

		if s.Logger != nil {
			s.Logger.Info("http server started", "addr", addr, "path", s.TelemetryPath)
		}

		go func(srv *http.Server, ln net.Listener) {
			err := srv.Serve(ln)
			if err == nil || err == http.ErrServerClosed {
				errCh <- nil
				return
			}
			errCh <- err
		}(srv, ln)
	}

	// Wait for shutdown or first error.
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
		// If one server exits cleanly unexpectedly, continue and wait for ctx.
		<-ctx.Done()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, srv := range servers {
		_ = srv.Shutdown(shutdownCtx)
	}

	return nil
}

