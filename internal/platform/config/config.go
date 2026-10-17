// Package config loads process configuration from environment variables.
//
// Business parameters (limits, thresholds) live in platform.settings, not here;
// this package only covers what a process needs to boot.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Env names the deployment environment. Only "local" and "test" exist (architecture §11).
type Env string

const (
	EnvLocal Env = "local"
	EnvTest  Env = "test"
)

type Config struct {
	Env         Env
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level

	WorkerID           string
	OutboxBatchSize    int
	OutboxPollInterval time.Duration
	OutboxMaxAttempts  int
}

// Load reads the environment, applies defaults and validates the result.
// All problems are reported together via errors.Join.
func Load() (Config, error) {
	var errs []error

	cfg := Config{
		Env:         Env(getenv("APP_ENV", string(EnvLocal))),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		WorkerID:    getenv("WORKER_ID", hostname()),
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	var err error
	if cfg.OutboxBatchSize, err = strconv.Atoi(getenv("OUTBOX_BATCH_SIZE", "20")); err != nil {
		errs = append(errs, fmt.Errorf("OUTBOX_BATCH_SIZE: %w", err))
	}
	if cfg.OutboxMaxAttempts, err = strconv.Atoi(getenv("OUTBOX_MAX_ATTEMPTS", "10")); err != nil {
		errs = append(errs, fmt.Errorf("OUTBOX_MAX_ATTEMPTS: %w", err))
	}
	if cfg.OutboxPollInterval, err = time.ParseDuration(getenv("OUTBOX_POLL_INTERVAL", "1s")); err != nil {
		errs = append(errs, fmt.Errorf("OUTBOX_POLL_INTERVAL: %w", err))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if cfg.Env != EnvLocal && cfg.Env != EnvTest {
		errs = append(errs, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvLocal, EnvTest, cfg.Env))
	}

	return cfg, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
