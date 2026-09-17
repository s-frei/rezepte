// Command rezepte runs the Rezepte service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/web"
)

// sweepInterval is how often expired sessions are swept from the database
// in the background, in addition to the sweep at boot.
const sweepInterval = 24 * time.Hour

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rezepte:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "rezepte.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}

	users := user.NewService(conn)
	if err := users.EnsureInitialAdmin(ctx, cfg.AdminUser, cfg.AdminPassword); err != nil {
		return err
	}
	sessions := auth.NewService(conn, users)
	if err := sessions.DeleteExpired(ctx); err != nil {
		logger.Warn("cleanup expired sessions", "err", err)
	}
	go sessions.SweepLoop(ctx, sweepInterval, logger)

	srv := httpserver.New(cfg, logger, web.Dist(), httpserver.WithAPIMiddleware(auth.Middleware(sessions, cfg.SecureCookies)))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	recipes := recipe.NewService(conn)
	recipe.Register(srv.API(), recipes)
	return srv.Run(ctx)
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
