package config

import (
	"log/slog"
	"testing"
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
