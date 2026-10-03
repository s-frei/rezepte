package config

import (
	"log/slog"
	"maps"
	"testing"

	"github.com/s-frei/rezepte/service/internal/mail"
)

func TestLoadFromDefaults(t *testing.T) {
	cfg, err := LoadFrom(map[string]string{})
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Addr != ":8060" {
		t.Errorf("Addr = %q, want :8060", cfg.Addr)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", cfg.DataDir)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want text", cfg.LogFormat)
	}
	if cfg.SecureCookies {
		t.Error("SecureCookies = true, want false")
	}
}

func TestLoadFromOverrides(t *testing.T) {
	cfg, err := LoadFrom(map[string]string{
		"REZEPTE_ADDR":           ":9000",
		"REZEPTE_DATA_DIR":       "/var/lib/rezepte",
		"REZEPTE_LOG_LEVEL":      "debug",
		"REZEPTE_LOG_FORMAT":     "json",
		"REZEPTE_SECURE_COOKIES": "true",
		"REZEPTE_ADMIN_USER":     "sam",
		"REZEPTE_ADMIN_PASSWORD": "secret",
	})
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Addr != ":9000" || cfg.DataDir != "/var/lib/rezepte" || !cfg.SecureCookies {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.LogLevel != slog.LevelDebug || cfg.LogFormat != "json" {
		t.Errorf("unexpected log config: %+v", cfg)
	}
	if cfg.AdminUser != "sam" || cfg.AdminPassword != "secret" {
		t.Errorf("unexpected admin config: %+v", cfg)
	}
}

func TestLoadFromRejectsBadLogFormat(t *testing.T) {
	if _, err := LoadFrom(map[string]string{"REZEPTE_LOG_FORMAT": "xml"}); err == nil {
		t.Fatal("expected error for log format xml")
	}
}

func TestLoadFromDefaultsToEnglish(t *testing.T) {
	cfg, err := LoadFrom(map[string]string{})
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Locale != "en" {
		t.Errorf("Locale = %q, want en", cfg.Locale)
	}
}

func TestLoadFromAcceptsGerman(t *testing.T) {
	cfg, err := LoadFrom(map[string]string{"REZEPTE_LOCALE": "de"})
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Locale != "de" {
		t.Errorf("Locale = %q, want de", cfg.Locale)
	}
}

func TestLoadFromRejectsUnknownLocale(t *testing.T) {
	if _, err := LoadFrom(map[string]string{"REZEPTE_LOCALE": "xx"}); err == nil {
		t.Fatal("LoadFrom accepted REZEPTE_LOCALE=xx")
	}
}

// TestLoadFromEmptyMeansDefault: a variable set to "" reads as unset.
func TestLoadFromEmptyMeansDefault(t *testing.T) {
	want, err := LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"REZEPTE_ADDR", "REZEPTE_DATA_DIR", "REZEPTE_ADMIN_USER", "REZEPTE_ADMIN_PASSWORD",
		"REZEPTE_LOG_LEVEL", "REZEPTE_LOG_FORMAT", "REZEPTE_LOCALE", "REZEPTE_SECURE_COOKIES",
		"REZEPTE_OIDC_NAME",
	} {
		got, err := LoadFrom(map[string]string{name: ""})
		if err != nil || got != want {
			t.Errorf("%s=\"\": %+v, %v; want the defaults %+v", name, got, err, want)
		}
	}
	if want.AdminUser != "admin" || want.AdminPassword != "" {
		t.Errorf("admin defaults = %q, %q", want.AdminUser, want.AdminPassword)
	}
}

// TestLoadFromParsesValuesVerbatim pins how values are read: strconv.ParseBool
// for booleans, slog.Level's own text form for the level, nothing trimmed.
func TestLoadFromParsesValuesVerbatim(t *testing.T) {
	for _, tc := range []struct {
		env   map[string]string
		check func(Config) bool
	}{
		{map[string]string{"REZEPTE_SECURE_COOKIES": "1"}, func(c Config) bool { return c.SecureCookies }},
		{map[string]string{"REZEPTE_SECURE_COOKIES": "TRUE"}, func(c Config) bool { return c.SecureCookies }},
		{map[string]string{"REZEPTE_SECURE_COOKIES": "false"}, func(c Config) bool { return !c.SecureCookies }},
		{map[string]string{"REZEPTE_LOG_LEVEL": "WARN"}, func(c Config) bool { return c.LogLevel == slog.LevelWarn }},
		{map[string]string{"REZEPTE_LOG_LEVEL": "info+2"}, func(c Config) bool { return c.LogLevel == slog.LevelInfo+2 }},
		{map[string]string{"REZEPTE_LOG_LEVEL": "error"}, func(c Config) bool { return c.LogLevel == slog.LevelError }},
		{map[string]string{"REZEPTE_ADDR": " :9000 "}, func(c Config) bool { return c.Addr == " :9000 " }},
	} {
		cfg, err := LoadFrom(tc.env)
		if err != nil || !tc.check(cfg) {
			t.Errorf("%v: %+v, %v", tc.env, cfg, err)
		}
	}
}

func TestLoadFromErrors(t *testing.T) {
	for _, tc := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"REZEPTE_SECURE_COOKIES": "yes"},
			`parse environment: env: parse error on field "SecureCookies" of type "bool": strconv.ParseBool: parsing "yes": invalid syntax`},
		{map[string]string{"REZEPTE_SECURE_COOKIES": " true"},
			`parse environment: env: parse error on field "SecureCookies" of type "bool": strconv.ParseBool: parsing " true": invalid syntax`},
		{map[string]string{"REZEPTE_LOG_LEVEL": "bogus"},
			`parse environment: env: parse error on field "LogLevel" of type "slog.Level": slog: level string "bogus": unknown name`},
		{map[string]string{"REZEPTE_LOG_LEVEL": " debug"},
			`parse environment: env: parse error on field "LogLevel" of type "slog.Level": slog: level string " debug": unknown name`},
		{map[string]string{"REZEPTE_LOG_LEVEL": "bogus", "REZEPTE_SECURE_COOKIES": "yes"},
			`parse environment: env: parse error on field "LogLevel" of type "slog.Level": slog: level string "bogus": unknown name; parse error on field "SecureCookies" of type "bool": strconv.ParseBool: parsing "yes": invalid syntax`},
		{map[string]string{"REZEPTE_LOG_FORMAT": "JSON"}, `REZEPTE_LOG_FORMAT must be text or json, got "JSON"`},
		{map[string]string{"REZEPTE_LOCALE": "DE"}, `REZEPTE_LOCALE must be one of [en de], got "DE"`},
		// A parse error wins over a bad format, as it did when the parser ran first.
		{map[string]string{"REZEPTE_LOG_FORMAT": "xml", "REZEPTE_SECURE_COOKIES": "yes"},
			`parse environment: env: parse error on field "SecureCookies" of type "bool": strconv.ParseBool: parsing "yes": invalid syntax`},
	} {
		cfg, err := LoadFrom(tc.env)
		if err == nil || err.Error() != tc.want || cfg != (Config{}) {
			t.Errorf("%v:\n got %+v, %v\nwant zero Config, %s", tc.env, cfg, err, tc.want)
		}
	}
}

func TestLoadReadsTheProcessEnvironment(t *testing.T) {
	t.Setenv("REZEPTE_ADDR", ":7000")
	t.Setenv("REZEPTE_LOCALE", "de")
	t.Setenv("REZEPTE_LOG_LEVEL", "")
	cfg, err := Load()
	if err != nil || cfg.Addr != ":7000" || cfg.Locale != "de" || cfg.LogLevel != slog.LevelInfo {
		t.Errorf("Load = %+v, %v", cfg, err)
	}
}

func TestLoadFromOIDC(t *testing.T) {
	all := map[string]string{
		"REZEPTE_OIDC_ISSUER":    "https://id.example",
		"REZEPTE_OIDC_CLIENT_ID": "rezepte",
		"REZEPTE_PUBLIC_URL":     "https://rezepte.example/",
	}
	cfg, err := LoadFrom(all)
	if err != nil || !cfg.OIDCEnabled() {
		t.Fatalf("all three set: %+v, %v", cfg, err)
	}
	if cfg.PublicURL != "https://rezepte.example" || cfg.OIDCName != "single sign-on" {
		t.Errorf("PublicURL = %q, OIDCName = %q", cfg.PublicURL, cfg.OIDCName)
	}
	if off, _ := LoadFrom(map[string]string{}); off.OIDCEnabled() {
		t.Error("enabled without configuration")
	}
	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{"issuer without public URL", map[string]string{"REZEPTE_OIDC_ISSUER": "https://id.example", "REZEPTE_OIDC_CLIENT_ID": "rezepte"}},
		{"client id alone", map[string]string{"REZEPTE_OIDC_CLIENT_ID": "rezepte", "REZEPTE_PUBLIC_URL": "https://rezepte.example"}},
		{"public URL with a path", map[string]string{"REZEPTE_OIDC_ISSUER": "https://id.example", "REZEPTE_OIDC_CLIENT_ID": "rezepte", "REZEPTE_PUBLIC_URL": "https://x.example/app"}},
		{"public URL with a query", map[string]string{"REZEPTE_PUBLIC_URL": "https://x.example/?a=b"}},
		{"public URL without scheme", map[string]string{"REZEPTE_PUBLIC_URL": "x.example"}},
	} {
		if cfg, err := LoadFrom(tc.env); err == nil {
			t.Errorf("%s: accepted %+v", tc.name, cfg)
		}
	}
}

func TestLoadFromSMTP(t *testing.T) {
	cfg, err := LoadFrom(map[string]string{
		"REZEPTE_PUBLIC_URL":    "https://rezepte.example.org",
		"REZEPTE_SMTP_HOST":     "smtp.example.org",
		"REZEPTE_SMTP_USERNAME": "rezepte",
		"REZEPTE_SMTP_PASSWORD": "secret",
		"REZEPTE_SMTP_FROM":     "rezepte@example.org",
	})
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	want := mail.Config{Host: "smtp.example.org", Port: 587, Security: mail.SecuritySTARTTLS,
		Username: "rezepte", Password: "secret", From: "rezepte@example.org", FromName: "Rezepte"}
	if cfg.SMTP != want {
		t.Fatalf("SMTP = %+v, want %+v", cfg.SMTP, want)
	}
	if !cfg.SMTP.Configured() {
		t.Fatal("SMTP not configured")
	}
}

func TestLoadFromSMTPUnsetIsZero(t *testing.T) {
	cfg, err := LoadFrom(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SMTP != (mail.Config{}) {
		t.Fatalf("SMTP = %+v, want zero", cfg.SMTP)
	}
}

func TestLoadFromSMTPRejects(t *testing.T) {
	base := map[string]string{"REZEPTE_PUBLIC_URL": "https://r.example.org", "REZEPTE_SMTP_HOST": "smtp.example.org", "REZEPTE_SMTP_FROM": "r@example.org"}
	for name, change := range map[string]map[string]string{
		"no from":           {"REZEPTE_SMTP_FROM": ""},
		"bad from":          {"REZEPTE_SMTP_FROM": "Rezepte <r@example.org>"},
		"bad security":      {"REZEPTE_SMTP_SECURITY": "ssl"},
		"bad port":          {"REZEPTE_SMTP_PORT": "0"},
		"port not a number": {"REZEPTE_SMTP_PORT": "abc"},
		"port too high":     {"REZEPTE_SMTP_PORT": "70000"},
		"no public url":     {"REZEPTE_PUBLIC_URL": ""},
	} {
		t.Run(name, func(t *testing.T) {
			env := maps.Clone(base)
			maps.Copy(env, change)
			if _, err := LoadFrom(env); err == nil {
				t.Fatal("LoadFrom accepted it")
			}
		})
	}
}
