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
	// ArtifactsDir stores run audio (local storage).
	ArtifactsDir string
	// WorkerConcurrency is how many runs one worker process executes at a time.
	WorkerConcurrency int
	// JudgeConcurrency is how many judgements one worker evaluates at a time.
	JudgeConcurrency int
	// JudgeTraceWait bounds how long the worker waits for Langfuse traces to settle.
	JudgeTraceWait time.Duration
	// JudgeTracePoll is how often the worker re-polls Langfuse while waiting.
	JudgeTracePoll time.Duration
}

// Load reads the configuration, applying development defaults.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:  env("DATABASE_URL", "postgres://testforge:testforge@localhost:5433/testforge?sslmode=disable"),
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		LogFormat:    env("LOG_FORMAT", "text"),
		SessionTTL:   30 * 24 * time.Hour,
		ArtifactsDir: env("ARTIFACTS_DIR", "./data/artifacts"),
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
	cfg.WorkerConcurrency = 3
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			return Config{}, fmt.Errorf("config WORKER_CONCURRENCY: use 1 to 50")
		}
		cfg.WorkerConcurrency = n
	}
	cfg.JudgeConcurrency = 2
	if v := os.Getenv("JUDGE_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 20 {
			return Config{}, fmt.Errorf("config JUDGE_CONCURRENCY: use 1 to 20")
		}
		cfg.JudgeConcurrency = n
	}
	cfg.JudgeTraceWait = 3 * time.Minute
	if v := os.Getenv("JUDGE_TRACE_WAIT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 || d > time.Hour {
			return Config{}, fmt.Errorf("config JUDGE_TRACE_WAIT: use a duration like 3m (at most 1h)")
		}
		cfg.JudgeTraceWait = d
	}
	cfg.JudgeTracePoll = 20 * time.Second
	if v := os.Getenv("JUDGE_TRACE_POLL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second || d > 10*time.Minute {
			return Config{}, fmt.Errorf("config JUDGE_TRACE_POLL: use a duration like 20s (1s to 10m)")
		}
		cfg.JudgeTracePoll = d
	}
	// Required by `serve` (validated when the server is built), not by `migrate`.
	cfg.SecretKey = os.Getenv("SECRET_KEY")
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
