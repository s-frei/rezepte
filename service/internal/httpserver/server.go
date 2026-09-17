// Package httpserver wires the HTTP API, static assets and server lifecycle.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/config"
)

// Server owns the mux, the huma API and the http.Server lifecycle.
type Server struct {
	cfg    config.Config
	logger *slog.Logger
	mux    *http.ServeMux
	api    huma.API
}

// New builds a server serving the API under /api/v1 and static assets from fsys.
func New(cfg config.Config, logger *slog.Logger, static fs.FS) *Server {
	mux := http.NewServeMux()
	s := &Server{cfg: cfg, logger: logger, mux: mux, api: newAPI(mux)}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/", SPAHandler(static))
	return s
}

// API exposes the huma API so feature packages can register operations.
func (s *Server) API() huma.API { return s.api }

// Handler returns the root handler, wrapped with request logging.
func (s *Server) Handler() http.Handler { return s.logRequests(s.mux) }

// Run serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		s.logger.Info("listening", "addr", s.cfg.Addr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen on %s: %w", s.cfg.Addr, err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Debug("request",
			"method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration", time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying writer so http.ResponseController can reach
// Flush, Hijack and deadline support through the logging middleware.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
