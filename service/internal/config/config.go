// Package config loads the service configuration from environment variables.
package config

import (
	"cmp"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/s-frei/rezepte/service/internal/user"
)

// Config holds every runtime setting. All values come from REZEPTE_* variables.
type Config struct {
	Addr          string     // REZEPTE_ADDR, default ":8060"
	DataDir       string     // REZEPTE_DATA_DIR, default "./data"
	AdminUser     string     // REZEPTE_ADMIN_USER, default "admin"
	AdminPassword string     // REZEPTE_ADMIN_PASSWORD, no default
	LogLevel      slog.Level // REZEPTE_LOG_LEVEL, default "info"
	LogFormat     string     // REZEPTE_LOG_FORMAT, default "text"
	Locale        string     // REZEPTE_LOCALE, default "en"
	SecureCookies bool       // REZEPTE_SECURE_COOKIES, default false

	PublicURL        string // REZEPTE_PUBLIC_URL, no default; stored without a trailing slash
	OIDCIssuer       string // REZEPTE_OIDC_ISSUER
	OIDCClientID     string // REZEPTE_OIDC_CLIENT_ID
	OIDCClientSecret string // REZEPTE_OIDC_CLIENT_SECRET, optional
	OIDCName         string // REZEPTE_OIDC_NAME, default "single sign-on"
}

// OIDCEnabled reports whether sign-in through an OpenID Connect provider is
// configured. load has already made sure the client id and public URL are
// set whenever the issuer is.
func (c Config) OIDCEnabled() bool { return c.OIDCIssuer != "" }

// Load reads the configuration from the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

// LoadFrom reads the configuration from the given map instead of the process
// environment. Missing keys fall back to defaults. Intended for tests.
func LoadFrom(environment map[string]string) (Config, error) {
	return load(func(key string) string { return environment[key] })
}

// load reads every variable through getenv. A variable set to "" counts as
// unset and gets its default; values are used verbatim, never trimmed.
func load(getenv func(string) string) (Config, error) {
	get := func(key, fallback string) string { return cmp.Or(getenv(key), fallback) }
	cfg := Config{
		Addr:          get("REZEPTE_ADDR", ":8060"),
		DataDir:       get("REZEPTE_DATA_DIR", "./data"),
		AdminUser:     get("REZEPTE_ADMIN_USER", "admin"),
		AdminPassword: getenv("REZEPTE_ADMIN_PASSWORD"),
		LogFormat:     get("REZEPTE_LOG_FORMAT", "text"),
		Locale:        get("REZEPTE_LOCALE", "en"),

		PublicURL:        getenv("REZEPTE_PUBLIC_URL"),
		OIDCIssuer:       getenv("REZEPTE_OIDC_ISSUER"),
		OIDCClientID:     getenv("REZEPTE_OIDC_CLIENT_ID"),
		OIDCClientSecret: getenv("REZEPTE_OIDC_CLIENT_SECRET"),
		OIDCName:         get("REZEPTE_OIDC_NAME", "single sign-on"),
	}
	// The messages keep the wording of the env library this replaced, which
	// names the Config field rather than the variable.
	var errs []string
	if err := cfg.LogLevel.UnmarshalText([]byte(get("REZEPTE_LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Sprintf(`parse error on field "LogLevel" of type "slog.Level": %v`, err))
	}
	secure, err := strconv.ParseBool(get("REZEPTE_SECURE_COOKIES", "false"))
	if err != nil {
		errs = append(errs, fmt.Sprintf(`parse error on field "SecureCookies" of type "bool": %v`, err))
	}
	cfg.SecureCookies = secure
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("parse environment: env: %s", strings.Join(errs, "; "))
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		return Config{}, fmt.Errorf("REZEPTE_LOG_FORMAT must be text or json, got %q", cfg.LogFormat)
	}
	if _, err := user.ParseLocale(cfg.Locale); err != nil {
		return Config{}, fmt.Errorf("REZEPTE_LOCALE must be one of %v, got %q", user.Locales, cfg.Locale)
	}
	if cfg.PublicURL != "" {
		public, err := parsePublicURL(cfg.PublicURL)
		if err != nil {
			return Config{}, err
		}
		cfg.PublicURL = public
	}
	if (cfg.OIDCIssuer != "" || cfg.OIDCClientID != "") &&
		(cfg.OIDCIssuer == "" || cfg.OIDCClientID == "" || cfg.PublicURL == "") {
		return Config{}, fmt.Errorf("REZEPTE_OIDC_ISSUER, REZEPTE_OIDC_CLIENT_ID and REZEPTE_PUBLIC_URL must be set together")
	}
	return cfg, nil
}

// parsePublicURL accepts an http(s) origin, optionally with a trailing slash,
// and returns it without one. Rezepte is served from the root of its host, so
// a path would only produce a redirect URI nothing answers.
func parsePublicURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("REZEPTE_PUBLIC_URL must be an http or https origin like https://rezepte.example.org, got %q", raw)
	}
	return strings.TrimSuffix(raw, "/"), nil
}
