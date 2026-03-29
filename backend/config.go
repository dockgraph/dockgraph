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
	BindAddr     string        // Listen address (DG_BIND_ADDR, default "0.0.0.0")
	Port         string        // HTTP listen port (DG_PORT, default "7800")
	PollInterval time.Duration // Docker API poll interval (DG_POLL_INTERVAL, default 30s)
	ComposePaths []string      // Explicit compose paths override (DG_COMPOSE_PATH); nil means auto-detect from container mounts
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() Config {
	cfg := Config{
		BindAddr:     "0.0.0.0",
		Port:         "7800",
		PollInterval: 30 * time.Second,
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

	return cfg
}
