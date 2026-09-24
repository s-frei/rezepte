// Command rezepte runs the Rezepte service.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
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
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/tokenapi"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
	"github.com/s-frei/rezepte/service/internal/web"
)

// sweepInterval is how often expired sessions are swept from the database
// in the background, in addition to the sweep at boot.
const sweepInterval = 24 * time.Hour

// version is the build version, set with -ldflags "-X main.version=...".
// A plain `go build` leaves it at "dev", which is what a development binary
// should report.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rezepte:", err)
		os.Exit(1)
	}
}

// healthcheck asks the instance listening on addr whether it is serving. It
// exists so the container image can declare a HEALTHCHECK: the runtime image is
// distroless, with no shell and no curl for one to call, so the binary has to
// be able to probe itself.
func healthcheck(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse %q: %w", addr, err)
	}
	// REZEPTE_ADDR is an address, not a port, and usually names every
	// interface (":8060"). The probe runs inside the same container, so it
	// asks the loopback one rather than guessing a routable address.
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	url := "http://" + net.JoinHostPort(host, port) + "/healthz"

	// Shorter than the HEALTHCHECK --timeout in the Dockerfile, so a hung
	// server is reported as unhealthy rather than killed mid-probe.
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("probe %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe %s: status %d", url, resp.StatusCode)
	}
	return nil
}

func run() error {
	demoMode := flag.Bool("demo", false, "fill an empty instance with sample recipes and, without REZEPTE_ADMIN_PASSWORD, a demo admin")
	showVersion := flag.Bool("version", false, "print the version and exit")
	probeHealth := flag.Bool("healthcheck", false, "probe this instance's own /healthz and exit non-zero if it does not answer")
	resetOwner := flag.Bool("reset-superadmin-password", false,
		"set the instance owner's password from REZEPTE_ADMIN_PASSWORD and exit; the server does not start")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Before anything touches the data directory or the database: this runs as
	// a second process inside a container whose first process owns both.
	if *probeHealth {
		return healthcheck(cfg.Addr)
	}

	demoDefaults := useDemoDefaults(*demoMode, *resetOwner, cfg.AdminPassword)
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

	users := user.NewService(conn, user.WithDefaultLocale(user.Locale(cfg.Locale)))
	if *resetOwner {
		owner, err := users.ResetSuperadminPassword(ctx, cfg.AdminPassword)
		if err != nil {
			return resetError(err, cfg.DataDir)
		}
		// Every device signed in with the old credential goes: a reset means
		// it is forgotten or compromised.
		if err := auth.NewService(conn, users).DeleteUserSessionsExcept(ctx, owner.ID, ""); err != nil {
			return err
		}
		logger.Info("superadmin password reset", "username", owner.Username)
		return nil
	}
	if err := users.EnsureSuperadmin(ctx, cfg.AdminUser, cfg.AdminPassword); err != nil {
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
	tokens := auth.NewTokenService(conn, users)

	srv := httpserver.New(cfg, logger, web.Dist(),
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, cfg.SecureCookies)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()),
		httpserver.WithSpecGuard(auth.RequireAuthOrLogin(sessions, tokens, cfg.SecureCookies)),
		httpserver.WithVersion(version))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	recipes := recipe.NewService(conn, recipe.WithImageDir(imageDir))
	recipe.Register(srv.API(), recipes)
	images := image.NewService(conn, imageDir)
	image.Register(srv.API(), images)
	settings.Register(srv.API(), settings.NewService(conn))
	srv.Handle("GET /images/{recipeId}/{imageId}/{file}",
		auth.RequireAuth(sessions, tokens, cfg.SecureCookies, auth.ScopeRecipesRead)(image.FileHandler(images)))
	userapi.Register(srv.API(), users, sessions)
	tokenapi.Register(srv.API(), tokens)
	return srv.Run(ctx)
}

// useDemoDefaults reports whether the well-known demo credentials stand in
// for the operator's configuration. Demo mode must work with no configuration
// at all, so they do whenever --demo is passed and no password is set - except
// during a reset, which would write the password published in the user docs
// onto the instance owner's account. A reset without REZEPTE_ADMIN_PASSWORD is
// refused with user.ErrAdminPasswordRequired instead, --demo or not.
func useDemoDefaults(demoMode, resetOwner bool, adminPassword string) bool {
	return demoMode && !resetOwner && adminPassword == ""
}

// resetErr carries an operator-facing message for the reset path while
// keeping the service's sentinel reachable. fmt.Errorf with %w would append
// the sentinel's own wording, which is written for the startup caller and
// tells someone who mistyped REZEPTE_DATA_DIR to recreate a database they
// have not lost.
type resetErr struct {
	msg string
	err error
}

func (e resetErr) Error() string { return e.msg }
func (e resetErr) Unwrap() error { return e.err }

// resetError restates the two sentinel errors the reset can return in the
// operator's terms. The service wording is written for the start path, where
// an owner-less database is a broken instance and a missing password is a
// first start; neither is true here. A mistyped REZEPTE_DATA_DIR is the
// likeliest reason the reset finds no owner, so the message names the
// directory it looked in rather than telling anyone to recreate a database.
// Anything else passes through untouched.
func resetError(err error, dataDir string) error {
	switch {
	case errors.Is(err, user.ErrNoSuperadmin):
		return resetErr{
			msg: fmt.Sprintf("no Rezepte instance with an owner at %s: check REZEPTE_DATA_DIR", dataDir),
			err: err,
		}
	case errors.Is(err, user.ErrAdminPasswordRequired):
		return resetErr{
			msg: "REZEPTE_ADMIN_PASSWORD is required to reset the owner's password",
			err: err,
		}
	}
	return err
}

// seedDemo fills an empty instance with the sample recipes and logs the
// demo admin's credentials so operators know how to log in. demoDefaults
// tells whether cfg.AdminUser/AdminPassword were replaced with the
// well-known demo credentials, or came from the operator's configuration.
func seedDemo(ctx context.Context, conn *sql.DB, imageDir string, cfg config.Config, demoDefaults bool, logger *slog.Logger) error {
	sum, err := demo.Seed(ctx, conn, imageDir, cfg.AdminUser, user.Locale(cfg.Locale), logger)
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
