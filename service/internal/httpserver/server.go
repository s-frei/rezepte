// Package httpserver wires the HTTP API, static assets and server lifecycle.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
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

// defaultNewError is huma's own error constructor, captured once so status
// codes below 500 keep their normal behaviour after New overrides the hook.
var (
	hideInternalErrorsOnce sync.Once
	defaultNewError        func(status int, msg string, errs ...error) huma.StatusError
)

// New builds a server serving the API under /api/v1 and static assets from fsys.
func New(cfg config.Config, logger *slog.Logger, static fs.FS) *Server {
	mux := http.NewServeMux()
	s := &Server{cfg: cfg, logger: logger, mux: mux, api: newAPI(mux)}

	// Override huma's error constructor so a plain error returned from a
	// handler (mapped by huma to a 500) never leaks its message - which may
	// contain internal details such as SQL driver errors - to the client.
	// The real constructor is captured only once so calling New repeatedly
	// (e.g. once per test) does not nest wrappers around itself.
	hideInternalErrorsOnce.Do(func() { defaultNewError = huma.NewError })
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		if status < http.StatusInternalServerError {
			return defaultNewError(status, msg, errs...)
		}
		s.logger.Error("internal error", "status", status, "err", errors.Join(errs...))
		return &huma.ErrorModel{
			Title:  http.StatusText(status),
			Status: status,
			Detail: "internal error",
		}
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	// huma's own operations register more specific patterns (e.g.
	// "GET /api/v1/openapi.json") which take precedence over these catch-alls.
	mux.Handle("/api", http.HandlerFunc(apiNotFound))
	mux.Handle("/api/", http.HandlerFunc(apiNotFound))
	mux.Handle("/", SPAHandler(static))
	return s
}

// apiNotFound answers unmatched /api routes with an RFC 9457 problem+json
// body instead of the stdlib's plain-text 404.
func apiNotFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(huma.ErrorModel{
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: "no such API route",
	})
}

// API exposes the huma API so feature packages can register operations.
func (s *Server) API() huma.API { return s.api }

// Handler returns the root handler with Origin check and request logging.
func (s *Server) Handler() http.Handler { return s.logRequests(checkOrigin(s.mux)) }

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

		level := slog.LevelDebug
		switch {
		case rec.status >= http.StatusInternalServerError:
			level = slog.LevelError
		case rec.status >= http.StatusBadRequest:
			level = slog.LevelWarn
		}
		s.logger.Log(r.Context(), level, "request",
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
