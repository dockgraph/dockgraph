package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port         string        // HTTP listen port (DG_PORT, default "7800")
	PollInterval time.Duration // Docker API poll interval (DG_POLL_INTERVAL, default 30s)
	ComposePaths []string      // Explicit compose paths override (DG_COMPOSE_PATH); nil means auto-detect from container mounts
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() Config {
	cfg := Config{
		Port:         "7800",
		PollInterval: 30 * time.Second,
	}

	if v := os.Getenv("DG_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			log.Printf("invalid DG_PORT %q, using default %s", v, cfg.Port)
		} else {
			cfg.Port = v
		}
	}
	if v := os.Getenv("DG_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PollInterval = d
		}
	}
	if v := os.Getenv("DG_COMPOSE_PATH"); v != "" {
		cfg.ComposePaths = strings.Split(v, ",")
	}

	return cfg
}
