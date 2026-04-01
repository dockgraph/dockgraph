package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dockgraph/dockgraph/auth"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	BindAddr     string        // Listen address (DG_BIND_ADDR, default "0.0.0.0")
	Port         string        // HTTP listen port (DG_PORT, default "7800")
	PollInterval time.Duration // Docker API poll interval (DG_POLL_INTERVAL, default 30s)
	ComposePaths []string      // Explicit compose paths override (DG_COMPOSE_PATH); nil means auto-detect from container mounts
	PasswordHash  string        // Argon2id hash of DG_PASSWORD; empty means auth disabled
	StatsInterval time.Duration // Container stats poll interval (DG_STATS_INTERVAL, default 3s)
	StatsWorkers  int           // Max concurrent stats API calls (DG_STATS_WORKERS, default 50)
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() Config {
	cfg := Config{
		BindAddr:     "0.0.0.0",
		Port:         "7800",
		PollInterval:  30 * time.Second,
		StatsInterval: 3 * time.Second,
		StatsWorkers:  50,
	}

	if v := os.Getenv("DG_BIND_ADDR"); v != "" {
		cfg.BindAddr = v
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
			if d < time.Second {
				log.Printf("DG_POLL_INTERVAL %s is too low, using minimum 1s", v)
				d = time.Second
			}
			cfg.PollInterval = d
		} else {
			log.Printf("invalid DG_POLL_INTERVAL %q, using default %s", v, cfg.PollInterval)
		}
	}
	if v := os.Getenv("DG_COMPOSE_PATH"); v != "" {
		parts := strings.Split(v, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		cfg.ComposePaths = parts
	}

	if v := os.Getenv("DG_STATS_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			if d < time.Second {
				log.Printf("DG_STATS_INTERVAL %s is too low, using minimum 1s", v)
				d = time.Second
			}
			cfg.StatsInterval = d
		} else {
			log.Printf("invalid DG_STATS_INTERVAL %q, using default %s", v, cfg.StatsInterval)
		}
	}
	if v := os.Getenv("DG_STATS_WORKERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			log.Printf("invalid DG_STATS_WORKERS %q, using default %d", v, cfg.StatsWorkers)
		} else {
			cfg.StatsWorkers = n
		}
	}

	if raw := os.Getenv("DG_PASSWORD"); raw != "" {
		if auth.IsHashedPassword(raw) {
			if err := auth.ValidateHash(raw); err != nil {
				log.Fatalf("DG_PASSWORD contains invalid hash: %v", err)
			}
			cfg.PasswordHash = raw
		} else {
			hash, err := auth.HashPassword(raw)
			if err != nil {
				log.Fatalf("Failed to hash DG_PASSWORD: %v", err)
			}
			cfg.PasswordHash = hash
		}
	}

	return cfg
}
