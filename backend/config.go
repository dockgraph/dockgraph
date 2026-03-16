package main

import (
	"os"
	"time"
)

type Config struct {
	Port         string
	PollInterval time.Duration
	ComposeDir   string
}

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
