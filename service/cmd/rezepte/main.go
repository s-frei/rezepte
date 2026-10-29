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

	// The zone database, so TZ works in the distroless image, which has
	// none: mails show times in the server's zone.
	_ "time/tzdata"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/avatar"
	"github.com/s-frei/rezepte/service/internal/config"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/demo"
	"github.com/s-frei/rezepte/service/internal/httpserver"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/mail"
	"github.com/s-frei/rezepte/service/internal/mailapi"
	"github.com/s-frei/rezepte/service/internal/mcpserver"
	"github.com/s-frei/rezepte/service/internal/notify"
	"github.com/s-frei/rezepte/service/internal/oidc"
	"github.com/s-frei/rezepte/service/internal/preview"
	"github.com/s-frei/rezepte/service/internal/push"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
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
// are deleted, and staged comment photos nobody sent. Expired links serve
// nothing from that moment on; the sweep only removes the rows.
const shareSweepInterval = time.Hour

// digestShutdownWait bounds how long a shutdown waits for a recipe digest
// run under way, so a hung SMTP server cannot keep the process alive.
const digestShutdownWait = 2 * time.Minute

// pushShutdownWait bounds how long a shutdown waits for a push under way;
// one send times out well before it.
const pushShutdownWait = 30 * time.Second

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
	demoOIDCIssuer := flag.String("demo-oidc-issuer", "", "with -demo, link the demo member jonas to his Dex identity at this issuer; the provider itself is configured through the settings API")
	demoNow := flag.String("demo-now", "", "with -demo, date the samples, their comments and the demo tokens from this RFC 3339 instant instead of now (for reproducible screenshots)")
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
		sessions := auth.NewService(conn, users)
		// Before exit, and before ending sessions can fail: the password is
		// written, and the notice is the owner's only sign that it happened.
		sessions.SetMail(mail.NewService(conn, cfg.SMTP, cfg.PublicURL, logger), nil)
		sessions.PasswordChanged(ctx, owner, mail.OriginCommandLine, "")
		if err := sessions.DeleteUserSessionsExcept(ctx, owner.ID, ""); err != nil {
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
		now := time.Now()
		if *demoNow != "" {
			if now, err = time.Parse(time.RFC3339, *demoNow); err != nil {
				return fmt.Errorf("-demo-now: %w", err)
			}
		}
		if err := seedDemo(ctx, conn, imageDir, cfg, demoDefaults, *demoOIDCIssuer, now, logger); err != nil {
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
	mailer := mail.NewService(conn, cfg.SMTP, cfg.PublicURL, logger)
	oidcSettings := oidc.NewSettings(conn, oidc.Config{
		PublicURL: cfg.PublicURL, Issuer: cfg.OIDCIssuer, ClientID: cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret, Name: cfg.OIDCName,
	})
	sessions.SetMail(mailer, oidcSettings.Provider)
	// Only log lines: both are resolved again on every request.
	if _, mailSource, err := mailer.Effective(ctx); err != nil {
		logger.Warn("mail", "err", err)
	} else {
		logger.Info("mail", "source", mailSource)
	}
	if _, src, err := oidcSettings.Effective(ctx); err != nil {
		logger.Warn("oidc", "err", err)
	} else {
		logger.Info("oidc", "source", src)
	}
	previews, err := preview.NewService(ctx, conn, instance, images, logger)
	if err != nil {
		return err
	}
	shares := share.NewService(conn, instance, users)
	// Link previews and unsubscribe links share the instance key; sign
	// labels keep one's token from verifying as the other's.
	signingKey, err := instance.LinkPreviewKey(ctx)
	if err != nil {
		return err
	}
	pushes := push.NewService(conn, cfg.PublicURL, &http.Client{})
	pushDone := pushes.Start(ctx)
	notifications := notify.NewService(conn, signingKey, users, avatars, images, mailer, pushes, cfg.PublicURL)
	sessions.SetNotifier(notifications)
	// One tick sweeps expired shares and unclaimed comment photos; the
	// photos also at start. A failure is logged, never the end of the loop.
	go func() {
		ticker := time.NewTicker(shareSweepInterval)
		defer ticker.Stop()
		for {
			if err := images.SweepCommentImages(ctx, time.Now()); err != nil {
				logger.Warn("sweep staged comment images", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if err := shares.Sweep(ctx, time.Now()); err != nil {
				logger.Warn("sweep expired shares", "err", err)
			}
		}
	}()
	mailer.SetReporter(notifications)
	digestDone := notifications.StartRecipeDigest(ctx)

	srv := httpserver.New(cfg, logger, web.Dist(),
		httpserver.WithAPIMiddleware(auth.Middleware(sessions, tokens, cfg.SecureCookies)),
		httpserver.WithSecuritySchemes(auth.SecuritySchemes()),
		httpserver.WithSpecGuard(auth.RequireAuthOrLogin(sessions, tokens, cfg.SecureCookies)),
		// A public share's page first: /s/{token} is never a recipe page.
		httpserver.WithLinkPreview(httpserver.PreviewFuncs(shares.ForRequest, previews.ForRequest)),
		httpserver.WithShellHeaders(share.ShellHeaders),
		httpserver.WithVersion(version))
	auth.Register(srv.API(), sessions, cfg.SecureCookies)
	oidc.Register(srv.API(), oidc.New(oidcSettings, sessions, users, cfg.SecureCookies))
	recipes := recipe.NewService(conn, imageDir)
	recipes.SetCommentNotifier(notifications)
	recipes.SetTastyNotifier(notifications)
	recipe.Register(srv.API(), recipes)
	recipeimport.Register(srv.API(), recipeimport.NewService(recipes, version, recipeimport.PublicOnly))
	uploadLinks := image.NewUploadLinks(cfg.PublicURL, users)
	photos := mcpserver.Photos{Images: images, Links: uploadLinks, Fetch: recipeimport.NewFetcher(version, recipeimport.PublicOnly)}
	if err := mcpserver.Register(srv.API(), recipes, version, photos); err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	image.Register(srv.API(), images)
	image.RegisterUploadLink(srv.API(), images, uploadLinks)
	avatar.Register(srv.API(), avatars)
	// Leftovers of a killed process are only wasted disk, so a failed sweep
	// is worth a warning, not a refusal to start.
	if err := transfer.SweepTemp(cfg.DataDir); err != nil {
		logger.Warn("transfer temp files", "err", err)
	}
	transfer.Register(srv.API(), transfer.NewService(recipes, images, cfg.DataDir, transfer.InputValidator(srv.API())))
	settings.Register(srv.API(), instance, mailer.Enabled)
	mailapi.Register(srv.API(), mailer)
	oidc.RegisterSettings(srv.API(), oidcSettings)
	preview.Register(srv.API(), previews)
	share.Register(srv.API(), shares)
	share.RegisterPublic(srv.API(), shares, recipes, images)
	notify.Register(srv.API(), notifications)
	push.Register(srv.API(), pushes)
	userapi.Register(srv.API(), users, sessions, avatars,
		func(ctx context.Context) string { return oidcSettings.Provider(ctx).Issuer }, mailer)
	tokenapi.Register(srv.API(), tokens)
	err = srv.Run(ctx)
	stop() // a failed listen returns before ctx ends
	// Both waits start now, so a stop takes the longer of the two, not their
	// sum: the stop timeouts in docker-compose.yaml and the docs count on it.
	pushDeadline := time.After(pushShutdownWait)
	select {
	case <-digestDone:
	case <-time.After(digestShutdownWait):
		logger.Warn("recipe digest still running at exit; its remaining mails are lost")
	}
	select {
	case <-pushDone:
	case <-pushDeadline:
		logger.Warn("push still sending at exit")
	}
	return err
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
// oidcIssuer is the issuer jonas's identity is linked at, empty for none.
// now is what the sample comments are dated back from.
func seedDemo(ctx context.Context, conn *sql.DB, imageDir string, cfg config.Config, demoDefaults bool, oidcIssuer string, now time.Time, logger *slog.Logger) error {
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
	sum, err := demo.Seed(ctx, conn, imageDir, cfg.AdminUser, members, locale, now, logger)
	if err != nil {
		return err
	}
	if demoDefaults {
		if err := demo.SeedMembers(ctx, conn, sum, cfg.AdminUser, oidcIssuer); err != nil {
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
