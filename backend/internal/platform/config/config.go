// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the API process configuration.
type Config struct {
	DatabaseURL  string
	HTTPAddr     string
	AppOrigins   []string
	CookieSecure bool
	AutoMigrate  bool
	LogFormat    string
	SessionTTL   time.Duration
	// SecretKey is the base64 32-byte key that encrypts provider credentials.
	SecretKey string
	// AllowPrivateNetwork lets provider URLs use http and private addresses
	// (local development only).
	AllowPrivateNetwork bool
}

// Load reads the configuration, applying development defaults.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: env("DATABASE_URL", "postgres://testforge:testforge@localhost:5433/testforge?sslmode=disable"),
		HTTPAddr:    env("HTTP_ADDR", ":8080"),
		LogFormat:   env("LOG_FORMAT", "text"),
		SessionTTL:  30 * 24 * time.Hour,
	}
	for _, o := range strings.Split(env("APP_ORIGINS", "http://localhost:3000"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.AppOrigins = append(cfg.AppOrigins, o)
		}
	}

	var err error
	if cfg.CookieSecure, err = boolEnv("COOKIE_SECURE", false); err != nil {
		return Config{}, err
	}
	if cfg.AutoMigrate, err = boolEnv("AUTO_MIGRATE", true); err != nil {
		return Config{}, err
	}
	if cfg.AllowPrivateNetwork, err = boolEnv("ALLOW_PRIVATE_NETWORK", false); err != nil {
		return Config{}, err
	}
	cfg.SecretKey = os.Getenv("SECRET_KEY")
	if cfg.SecretKey == "" {
		return Config{}, fmt.Errorf("config SECRET_KEY is required: a base64 32-byte key, e.g. `openssl rand -base64 32`")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config %s: %w", key, err)
	}
	return b, nil
}
