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
	"sync/atomic"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/config"
)

// Server owns the mux, the huma API and the http.Server lifecycle.
type Server struct {
	cfg       config.Config
	logger    *slog.Logger
	mux       *http.ServeMux
	api       huma.API
	specGuard func(http.Handler) http.Handler
}

// Option configures a Server at construction time. Options run after the
// huma API is created but before New returns, so an Option such as
// WithAPIMiddleware is guaranteed to apply before the caller can register
// any operation on the returned Server.
type Option func(*Server)

// WithAPIMiddleware installs an API-wide huma middleware. mw receives the
// server's huma.API (useful for writing errors with huma.WriteErr) and
// returns the actual middleware function.
//
// Because New applies every Option before returning, a middleware installed
// this way is structurally guaranteed to run for every operation registered
// afterwards - there is no window in which an operation could be registered
// ahead of it, unlike calling api.UseMiddleware from arbitrary caller code.
func WithAPIMiddleware(mw func(api huma.API) func(huma.Context, func(huma.Context))) Option {
	return func(s *Server) {
		s.api.UseMiddleware(mw(s.api))
	}
}

// WithSpecGuard wraps the routes huma registers for the contract itself -
// the OpenAPI document, the JSON schemas and the Scalar docs page.
//
// Those are not huma operations: huma hangs them straight off the mux, so
// the middleware WithAPIMiddleware installs never runs for them and their
// Security is empty. Guarding them therefore has to happen here, in front
// of the mux, rather than in the operation chain. mw is an ordinary
// net/http middleware - auth.RequireSession, the same one the image routes
// use.
func WithSpecGuard(mw func(http.Handler) http.Handler) Option {
	return func(s *Server) {
		s.specGuard = mw
	}
}

// errorLogger holds the logger the huma error hook below logs to. New stores
// its logger here; when multiple Servers exist, the last call to New wins
// for every server's errors, since the hook is a single package-level
// override of huma.NewErrorWithContext. This is fine in this process, which
// only ever runs one Server, and is acceptable in tests, which each build
// their own short-lived Server sequentially.
var errorLogger atomic.Pointer[slog.Logger]

// installErrorHookOnce guards the one-time capture of huma's original
// NewErrorWithContext and installation of the replacement below.
var installErrorHookOnce sync.Once

// installErrorHook overrides huma.NewErrorWithContext - the constructor huma
// calls both for handler errors and from huma.WriteErr - so that a plain
// error returned from a handler (mapped by huma to a 500) never leaks its
// message (which may contain internal details such as SQL driver errors) to
// the client. Status codes below 500 keep huma's normal behaviour via the
// captured original constructor.
func installErrorHook() {
	installErrorHookOnce.Do(func() {
		original := huma.NewErrorWithContext
		huma.NewErrorWithContext = func(ctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
			if status < http.StatusInternalServerError {
				return original(ctx, status, msg, errs...)
			}
			if logger := errorLogger.Load(); logger != nil {
				logger.Error("internal error",
					"operation", ctx.Operation().OperationID,
					"method", ctx.Method(),
					"path", ctx.URL().Path,
					"err", errors.Join(errs...))
			}
			return &huma.ErrorModel{
				Title:  http.StatusText(status),
				Status: status,
				Detail: "internal error",
			}
		}
	})
}

// New builds a server serving the API under /api/v1 and static assets from
// fsys. Options run after the huma API is created and before any operation
// can be registered on it; use WithAPIMiddleware to install API-wide
// middleware (such as auth.Middleware) with that guarantee.
func New(cfg config.Config, logger *slog.Logger, static fs.FS, opts ...Option) *Server {
	mux := http.NewServeMux()
	s := &Server{cfg: cfg, logger: logger, mux: mux, api: newAPI(mux)}

	installErrorHook()
	errorLogger.Store(logger)

	for _, opt := range opts {
		opt(s)
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

// Handle registers h on the server's mux for a route that is not a huma
// operation (Phase 4's image files). Go 1.22 mux precedence picks the most
// specific matching pattern, so a pattern such as
// "GET /images/{recipeId}/{imageId}/{file}" wins over the "/" SPA
// catch-all New installed, whatever the registration order. The handler is
// still wrapped by Handler()'s Origin check and request logging.
func (s *Server) Handle(pattern string, h http.Handler) {
	s.mux.Handle(pattern, h)
}

// Handler returns the root handler with Origin check and request logging.
func (s *Server) Handler() http.Handler { return s.logRequests(checkOrigin(s.guardSpec(s.mux))) }

// guardSpec routes the contract paths through the middleware WithSpecGuard
// installed and lets everything else reach the mux untouched. With no guard
// configured the mux is returned as it is, so a Server built without the
// option behaves exactly as before.
func (s *Server) guardSpec(next http.Handler) http.Handler {
	if s.specGuard == nil {
		return next
	}
	guarded := s.specGuard(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSpecRoute(r.URL.Path) {
			guarded.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
