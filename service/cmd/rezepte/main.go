// Command rezepte runs the Rezepte service.
package main

import (
	"context"
	"database/sql"
	"flag"
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
	"github.com/s-frei/rezepte/service/internal/demo"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
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
	demoMode := flag.Bool("demo", false, "fill an empty instance with sample recipes and, without REZEPTE_ADMIN_PASSWORD, a demo admin")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Demo mode must work with no configuration at all: fall back to the
	// well-known demo admin only when the operator set no password.
	demoDefaults := *demoMode && cfg.AdminPassword == ""
	if demoDefaults {
		cfg.AdminUser, cfg.AdminPassword = demo.AdminUser, demo.AdminPassword
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
	imageDir := filepath.Join(cfg.DataDir, "images")
	if *demoMode {
		if err := seedDemo(ctx, conn, imageDir, cfg, demoDefaults, logger); err != nil {
			return err
		}
	}

	sessions := auth.NewService(conn, users)
	if err := sessions.DeleteExpired(ctx); err != nil {
		logger.Warn("cleanup expired sessions", "err", err)
	}
	go sessions.SweepLoop(ctx, sweepInterval, logger)

	srv := httpserver.New(cfg, logger, web.Dist(),
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, cfg.SecureCookies)),
		httpserver.WithSpecGuard(auth.RequireSessionOrLogin(sessions, cfg.SecureCookies)))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	recipes := recipe.NewService(conn, recipe.WithImageDir(imageDir))
	recipe.Register(srv.API(), recipes)
	images := image.NewService(conn, imageDir)
	image.Register(srv.API(), images)
	srv.Handle("GET /images/{recipeId}/{imageId}/{file}",
		auth.RequireSession(sessions, cfg.SecureCookies)(image.FileHandler(images)))
	userapi.Register(srv.API(), users, sessions)
	return srv.Run(ctx)
}

// seedDemo fills an empty instance with the sample recipes and logs the
// demo admin's credentials so operators know how to log in. demoDefaults
// tells whether cfg.AdminUser/AdminPassword were replaced with the
// well-known demo credentials, or came from the operator's configuration.
func seedDemo(ctx context.Context, conn *sql.DB, imageDir string, cfg config.Config, demoDefaults bool, logger *slog.Logger) error {
	sum, err := demo.Seed(ctx, conn, imageDir, cfg.AdminUser, logger)
	if err != nil {
		return err
	}
	password := "from REZEPTE_ADMIN_PASSWORD" //nolint:gosec // G101: log label, not a credential
	if demoDefaults {
		password = demo.AdminPassword
	}
	logger.Info("demo mode", "user", cfg.AdminUser, "password", password, "seeded", !sum.Skipped, "dataDir", cfg.DataDir)
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
