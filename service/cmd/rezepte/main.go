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
	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/demo"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mcpserver"
	"github.com/s-frei/rezepte/service/internal/oidc"
	"github.com/s-frei/rezepte/service/internal/preview"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/settings"
	"github.com/s-frei/rezepte/service/internal/share"
	"github.com/s-frei/rezepte/service/internal/tokenapi"
	"github.com/s-frei/rezepte/service/internal/transfer"
	"github.com/s-frei/rezepte/service/internal/user"
	"github.com/s-frei/rezepte/service/internal/userapi"
	"github.com/s-frei/rezepte/service/internal/web"
)

// sweepInterval is how often expired sessions are swept from the database
// in the background, in addition to the sweep at boot.
const sweepInterval = 24 * time.Hour

// shareSweepInterval is how often public links whose own expiry has passed
// are deleted. They serve nothing from that moment on; the sweep only
// removes the rows.
const shareSweepInterval = time.Hour

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

	users := user.NewService(conn, user.Locale(cfg.Locale))
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
	images := image.NewService(conn, imageDir)
	avatars := avatar.NewService(conn, filepath.Join(cfg.DataDir, "avatars"), images)
	instance := settings.NewService(conn)
	previews, err := preview.NewService(ctx, conn, instance, images, logger)
	if err != nil {
		return err
	}
	shares := share.NewService(conn, instance, users)
	go shares.SweepLoop(ctx, shareSweepInterval, logger)

	srv := httpserver.New(cfg, logger, web.Dist(),
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, cfg.SecureCookies)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()),
		httpserver.WithSpecGuard(auth.RequireAuthOrLogin(sessions, tokens, cfg.SecureCookies)),
		// A public share's page first: /s/{token} is never a recipe page.
		httpserver.WithLinkPreview(httpserver.PreviewFuncs(shares.ForRequest, previews.ForRequest)),
		httpserver.WithShellHeaders(share.ShellHeaders),
		httpserver.WithVersion(version))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	var login *oidc.Login
	if cfg.OIDCEnabled() {
		login = oidc.New(oidc.Config{
			PublicURL: cfg.PublicURL, Issuer: cfg.OIDCIssuer, ClientID: cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret, Name: cfg.OIDCName,
		}, sessions, users, cfg.SecureCookies)
	}
	oidc.Register(srv.API(), login)
	recipes := recipe.NewService(conn, imageDir)
	recipe.Register(srv.API(), recipes)
	if err := mcpserver.Register(srv.API(), recipes, version); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	image.Register(srv.API(), images)
	avatar.Register(srv.API(), avatars)
	// Leftovers of a killed process are only wasted disk, so a failed sweep
	// is worth a warning, not a refusal to start.
	if err := transfer.SweepTemp(cfg.DataDir); err != nil {
		logger.Warn("transfer temp files", "err", err)
	}
	transfer.Register(srv.API(), transfer.NewService(recipes, images, cfg.DataDir, transfer.InputValidator(srv.API())))
	settings.Register(srv.API(), instance)
	preview.Register(srv.API(), previews)
	share.Register(srv.API(), shares)
	share.RegisterPublic(srv.API(), shares, recipes, images)
	userapi.Register(srv.API(), users, sessions, avatars, cfg.OIDCIssuer)
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

// seedDemo fills an empty instance with the sample recipes - and, on the
// demo credentials, with the demo's other members, the samples they wrote
// and their tasty marks - and logs the demo admin's credentials so operators
// know how to log in. demoDefaults
// tells whether cfg.AdminUser/AdminPassword were replaced with the
// well-known demo credentials, or came from the operator's configuration.
func seedDemo(ctx context.Context, conn *sql.DB, imageDir string, cfg config.Config, demoDefaults bool, logger *slog.Logger) error {
	locale := user.Locale(cfg.Locale)
	password := "from REZEPTE_ADMIN_PASSWORD" //nolint:gosec // G101: log label, not a credential
	var members []user.User
	if demoDefaults {
		password = demo.AdminPassword
		// The members' passwords are as public as the admin's, so they come
		// only with the published credentials, never beside an operator's own.
		// They exist before the seed, which writes some samples as theirs.
		var err error
		if members, err = demo.AddMembers(ctx, conn, locale, logger); err != nil {
			return err
		}
	}
	sum, err := demo.Seed(ctx, conn, imageDir, cfg.AdminUser, members, locale, logger)
	if err != nil {
		return err
	}
	if demoDefaults {
		if err := demo.SeedMembers(ctx, conn, sum, cfg.AdminUser, cfg.OIDCIssuer); err != nil {
			return err
		}
		avatars := avatar.NewService(conn, filepath.Join(cfg.DataDir, "avatars"), image.NewService(conn, imageDir))
		if err := demo.SeedAvatars(ctx, conn, avatars, logger); err != nil {
			return err
		}
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
