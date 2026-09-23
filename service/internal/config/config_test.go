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
