// Package config loads KutubCore configuration from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration. No secrets are read from
// the database; they come from the environment only (see blueprint §14).
type Config struct {
	Addr         string
	DBPath       string
	LogLevel     string
	HTTPTimeouts timeouts
}

type timeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
}

// Load reads configuration from the environment, falling back to
// standalone defaults (Blueprint §2: standalone Go + SQLite on one machine).
func Load() Config {
	return Config{
		Addr:     envOr("ADDR", ":8080"),
		DBPath:   envOr("DB_PATH", "data/kutubcore.db"),
		LogLevel: envOr("LOG_LEVEL", "info"),
		HTTPTimeouts: timeouts{
			ReadHeader: time.Duration(envIntOr("HTTP_READ_HEADER_TIMEOUT_SEC", 5)) * time.Second,
			Read:       time.Duration(envIntOr("HTTP_READ_TIMEOUT_SEC", 15)) * time.Second,
			Write:      time.Duration(envIntOr("HTTP_WRITE_TIMEOUT_SEC", 15)) * time.Second,
			Idle:       time.Duration(envIntOr("HTTP_IDLE_TIMEOUT_SEC", 60)) * time.Second,
		},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// ReadHeaderTimeout exposes the read-header timeout for the HTTP server.
func (c Config) ReadHeaderTimeout() time.Duration { return c.HTTPTimeouts.ReadHeader }
func (c Config) ReadTimeout() time.Duration       { return c.HTTPTimeouts.Read }
func (c Config) WriteTimeout() time.Duration      { return c.HTTPTimeouts.Write }
func (c Config) IdleTimeout() time.Duration       { return c.HTTPTimeouts.Idle }
