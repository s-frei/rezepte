// Package config loads the service configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"

	"github.com/caarlos0/env/v11"
)

// Config holds every runtime setting. All values come from REZEPTE_* variables.
type Config struct {
	Addr          string     `env:"REZEPTE_ADDR" envDefault:":8080"`
	DataDir       string     `env:"REZEPTE_DATA_DIR" envDefault:"./data"`
	AdminUser     string     `env:"REZEPTE_ADMIN_USER" envDefault:"admin"`
	AdminPassword string     `env:"REZEPTE_ADMIN_PASSWORD"`
	LogLevel      slog.Level `env:"REZEPTE_LOG_LEVEL" envDefault:"info"`
	LogFormat     string     `env:"REZEPTE_LOG_FORMAT" envDefault:"text"`
	SecureCookies bool       `env:"REZEPTE_SECURE_COOKIES" envDefault:"false"`
}

// Load reads the configuration from the process environment.
func Load() (Config, error) {
	return parse(env.Options{})
}

// LoadFrom reads the configuration from the given map instead of the process
// environment. Missing keys fall back to defaults. Intended for tests.
func LoadFrom(environment map[string]string) (Config, error) {
	return parse(env.Options{Environment: environment})
}

func parse(opts env.Options) (Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](opts)
	if err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		return Config{}, fmt.Errorf("REZEPTE_LOG_FORMAT must be text or json, got %q", cfg.LogFormat)
	}
	return cfg, nil
}
