package main

import (
	"os"
	"time"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port         string        // HTTP listen port (DF_PORT, default "7800")
	PollInterval time.Duration // Docker API poll interval (DF_POLL_INTERVAL, default 30s)
	ComposeDir   string        // Directory to scan for compose files (DF_COMPOSE_DIR, default "/app/compose")
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() Config {
	cfg := Config{
		Port:         "7800",
		PollInterval: 30 * time.Second,
		ComposeDir:   "/app/compose",
	}

	if v := os.Getenv("DF_PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("DF_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PollInterval = d
		}
	}
	if v := os.Getenv("DF_COMPOSE_DIR"); v != "" {
		cfg.ComposeDir = v
	}

	return cfg
}
