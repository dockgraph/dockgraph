package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dockgraph/dockgraph/auth"
	"github.com/dockgraph/dockgraph/collector"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	BindAddr      string        // Listen address (DG_BIND_ADDR, default "0.0.0.0")
	Port          string        // HTTP listen port (DG_PORT, default "7800")
	PollInterval  time.Duration // Docker API poll interval (DG_POLL_INTERVAL, default 30s)
	ComposePaths  []string      // Explicit compose paths override (DG_COMPOSE_PATH); nil means auto-detect from container mounts
	PasswordHash  string        // Argon2id hash of DG_PASSWORD; empty means auth disabled
	StatsInterval time.Duration // Container stats poll interval (DG_STATS_INTERVAL, default 3s)
	StatsWorkers  int           // Max concurrent stats API calls (DG_STATS_WORKERS, default 50)
	// Mode selects the runtime mode (DG_MODE: auto|standalone|swarm|agent,
	// default auto). Auto resolves at startup from the daemon's swarm state.
	Mode string
	// SwarmPollInterval is how often swarm tasks are polled for changes on
	// remote nodes (DG_SWARM_POLL_INTERVAL, default 5s).
	SwarmPollInterval time.Duration
	// AgentPort is the per-node agent's HTTP port (DG_AGENT_PORT, default
	// 7801). Agents listen on it; the swarm server dials agents on it.
	AgentPort string
	// AgentAddr is the DNS name resolving to every agent task
	// (DG_AGENT_ADDR, default "tasks.agent"), optionally with ":port".
	AgentAddr string
	// AgentToken is the shared bearer secret between server and agents,
	// read from DG_AGENT_TOKEN or the file named by DG_AGENT_TOKEN_FILE
	// (a Docker secret). Empty disables agents on the server and prevents
	// an agent from starting.
	AgentToken string
}

// validModes lists the accepted DG_MODE values.
var validModes = map[string]bool{
	collector.ModeAuto:       true,
	collector.ModeStandalone: true,
	collector.ModeSwarm:      true,
	collector.ModeAgent:      true,
}

// LoadConfig reads configuration from environment variables with sensible defaults.
// Default listen ports of the server and the per-node agent.
const (
	defaultPort      = "7800"
	defaultAgentPort = "7801"
)

func LoadConfig() Config {
	cfg := Config{
		BindAddr:      "0.0.0.0",
		Port:          defaultPort,
		PollInterval:  30 * time.Second,
		StatsInterval: 3 * time.Second,
		StatsWorkers:  50,

		Mode:              collector.ModeAuto,
		SwarmPollInterval: 5 * time.Second,
		AgentPort:         defaultAgentPort,
		AgentAddr:         "tasks.agent",
	}

	if v := os.Getenv("DG_BIND_ADDR"); v != "" {
		cfg.BindAddr = v
	}
	cfg.Port = parsePort("DG_PORT", cfg.Port)
	cfg.AgentPort = parsePort("DG_AGENT_PORT", cfg.AgentPort)
	if v := strings.TrimSpace(os.Getenv("DG_AGENT_ADDR")); v != "" {
		cfg.AgentAddr = v
	}
	cfg.AgentToken = loadAgentToken()

	cfg.PollInterval = parseDuration("DG_POLL_INTERVAL", cfg.PollInterval, time.Second)
	cfg.StatsInterval = parseDuration("DG_STATS_INTERVAL", cfg.StatsInterval, time.Second)
	cfg.SwarmPollInterval = parseDuration("DG_SWARM_POLL_INTERVAL", cfg.SwarmPollInterval, time.Second)

	cfg.Mode = parseMode()

	if v := os.Getenv("DG_COMPOSE_PATH"); v != "" {
		parts := strings.Split(v, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		cfg.ComposePaths = parts
	}

	if v := os.Getenv("DG_STATS_WORKERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			log.Printf("invalid DG_STATS_WORKERS %q, using default %d", v, cfg.StatsWorkers)
		} else {
			cfg.StatsWorkers = n
		}
	}

	raw, err := readSecretEnv("DG_PASSWORD")
	if err != nil {
		// A misconfigured secret must never silently disable authentication.
		log.Fatalf("%v", err)
	}
	if raw != "" {
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

// parseMode reads DG_MODE, keeping the default (auto) for an empty or
// invalid value.
func parseMode() string {
	v := os.Getenv("DG_MODE")
	if v == "" {
		return collector.ModeAuto
	}
	mode := strings.ToLower(strings.TrimSpace(v))
	if !validModes[mode] {
		log.Printf("invalid DG_MODE %q, using default %s", v, collector.ModeAuto)
		return collector.ModeAuto
	}
	return mode
}

// healthcheckPort returns the port --healthcheck probes: the agent's in
// agent mode, the server's otherwise. It reads only DG_MODE and the ports,
// unlike LoadConfig, so a probe never reads secrets or hashes the password.
func healthcheckPort() string {
	if parseMode() == collector.ModeAgent {
		return parsePort("DG_AGENT_PORT", defaultAgentPort)
	}
	return parsePort("DG_PORT", defaultPort)
}

// parsePort reads a TCP port from an environment variable, keeping fallback
// when the value is not a valid port number.
func parsePort(envKey, fallback string) string {
	v := os.Getenv(envKey)
	if v == "" {
		return fallback
	}
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		log.Printf("invalid %s %q, using default %s", envKey, v, fallback)
		return fallback
	}
	return v
}

// readSecretEnv returns the value of the environment variable key, or the
// contents of the file named by key+"_FILE" (surrounding whitespace, such as
// the trailing newline of a Docker secret, is trimmed). The inline variable
// wins when both are set. It fails when the file is named but unreadable or
// empty (whitespace only).
func readSecretEnv(key string) (string, error) {
	value := os.Getenv(key)
	path := strings.TrimSpace(os.Getenv(key + "_FILE"))
	if value != "" {
		if path != "" {
			log.Printf("WARN  both %s and %s_FILE are set, using %s", key, key, key)
		}
		return value, nil
	}
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read %s_FILE: %w", key, err)
	}
	// A configured but empty secret file is a misconfiguration, not "unset":
	// for DG_PASSWORD it would otherwise silently disable authentication.
	value = strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("%s_FILE is empty: %s", key, path)
	}
	return value, nil
}

// loadAgentToken returns the agent shared secret from DG_AGENT_TOKEN or
// DG_AGENT_TOKEN_FILE (see readSecretEnv). An unreadable file yields an empty
// token, which agent mode refuses.
func loadAgentToken() string {
	token, err := readSecretEnv("DG_AGENT_TOKEN")
	if err != nil {
		log.Printf("WARN  %v", err)
		return ""
	}
	return strings.TrimSpace(token)
}

// parseDuration reads a duration from an environment variable with minimum enforcement.
func parseDuration(envKey string, fallback, minimum time.Duration) time.Duration {
	v := os.Getenv(envKey)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("invalid %s %q, using default %s", envKey, v, fallback)
		return fallback
	}
	if d < minimum {
		log.Printf("%s %s is too low, using minimum %s", envKey, v, minimum)
		return minimum
	}
	return d
}
