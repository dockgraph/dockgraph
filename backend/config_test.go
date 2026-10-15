package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dockgraph/dockgraph/auth"
)

// ---------------------------------------------------------------------------
// parseDuration
// ---------------------------------------------------------------------------

func TestParseDuration_ValidString(t *testing.T) {
	t.Setenv("TEST_DUR", "15s")
	got := parseDuration("TEST_DUR", 30*time.Second, time.Second)
	if got != 15*time.Second {
		t.Errorf("expected 15s, got %v", got)
	}
}

func TestParseDuration_EmptyFallback(t *testing.T) {
	// Ensure the env var is not set (t.Setenv with cleanup handles this, but
	// for an unset var we just don't set it).
	t.Setenv("TEST_DUR_EMPTY", "")
	got := parseDuration("TEST_DUR_UNSET_KEY", 5*time.Second, time.Second)
	if got != 5*time.Second {
		t.Errorf("expected fallback 5s, got %v", got)
	}
}

func TestParseDuration_InvalidFallback(t *testing.T) {
	t.Setenv("TEST_DUR_BAD", "notaduration")
	got := parseDuration("TEST_DUR_BAD", 20*time.Second, time.Second)
	if got != 20*time.Second {
		t.Errorf("expected fallback 20s after invalid input, got %v", got)
	}
}

func TestParseDuration_BelowMinimumClamped(t *testing.T) {
	t.Setenv("TEST_DUR_LOW", "200ms")
	got := parseDuration("TEST_DUR_LOW", 30*time.Second, time.Second)
	if got != time.Second {
		t.Errorf("expected minimum 1s, got %v", got)
	}
}

func TestParseDuration_AtMinimum(t *testing.T) {
	t.Setenv("TEST_DUR_MIN", "1s")
	got := parseDuration("TEST_DUR_MIN", 30*time.Second, time.Second)
	if got != time.Second {
		t.Errorf("expected 1s (at minimum), got %v", got)
	}
}

func TestParseDuration_AboveMinimum(t *testing.T) {
	t.Setenv("TEST_DUR_HIGH", "2m")
	got := parseDuration("TEST_DUR_HIGH", 30*time.Second, time.Second)
	if got != 2*time.Minute {
		t.Errorf("expected 2m, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// LoadConfig — defaults
// ---------------------------------------------------------------------------

func TestLoadConfig_Defaults(t *testing.T) {
	// Ensure none of the config env vars are set.
	for _, key := range []string{
		"DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
		"DG_COMPOSE_PATH", "DG_PASSWORD", "DG_STATS_INTERVAL",
		"DG_STATS_WORKERS", "DG_MODE", "DG_SWARM_POLL_INTERVAL",
		"DG_AGENT_PORT", "DG_AGENT_ADDR", "DG_AGENT_TOKEN", "DG_AGENT_TOKEN_FILE",
	} {
		t.Setenv(key, "")
	}
	// t.Setenv("X", "") sets the var to empty, but LoadConfig checks for
	// non-empty, so this is equivalent to unset for our purposes.

	cfg := LoadConfig()

	if cfg.BindAddr != "0.0.0.0" {
		t.Errorf("BindAddr: got %s, want 0.0.0.0", cfg.BindAddr)
	}
	if cfg.Port != "7800" {
		t.Errorf("Port: got %s, want 7800", cfg.Port)
	}
	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval: got %v, want 30s", cfg.PollInterval)
	}
	if cfg.StatsInterval != 3*time.Second {
		t.Errorf("StatsInterval: got %v, want 3s", cfg.StatsInterval)
	}
	if cfg.StatsWorkers != 50 {
		t.Errorf("StatsWorkers: got %d, want 50", cfg.StatsWorkers)
	}
	if cfg.ComposePaths != nil {
		t.Errorf("ComposePaths: got %v, want nil", cfg.ComposePaths)
	}
	if cfg.PasswordHash != "" {
		t.Errorf("PasswordHash: got %q, want empty", cfg.PasswordHash)
	}
	if cfg.Mode != "auto" {
		t.Errorf("Mode: got %s, want auto", cfg.Mode)
	}
	if cfg.SwarmPollInterval != 5*time.Second {
		t.Errorf("SwarmPollInterval: got %v, want 5s", cfg.SwarmPollInterval)
	}
	if cfg.AgentPort != "7801" {
		t.Errorf("AgentPort: got %s, want 7801", cfg.AgentPort)
	}
	if cfg.AgentAddr != "tasks.agent" {
		t.Errorf("AgentAddr: got %s, want tasks.agent", cfg.AgentAddr)
	}
	if cfg.AgentToken != "" {
		t.Errorf("AgentToken: got %q, want empty", cfg.AgentToken)
	}
}

// ---------------------------------------------------------------------------
// DG_BIND_ADDR
// ---------------------------------------------------------------------------

func TestLoadConfig_BindAddr(t *testing.T) {
	t.Setenv("DG_BIND_ADDR", "127.0.0.1")
	clearConfigEnv(t, "DG_PORT", "DG_POLL_INTERVAL", "DG_COMPOSE_PATH",
		"DG_PASSWORD", "DG_STATS_INTERVAL", "DG_STATS_WORKERS")

	cfg := LoadConfig()
	if cfg.BindAddr != "127.0.0.1" {
		t.Errorf("got %s, want 127.0.0.1", cfg.BindAddr)
	}
}

// ---------------------------------------------------------------------------
// DG_PORT
// ---------------------------------------------------------------------------

func TestLoadConfig_Port(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"valid_8080", "8080", "8080"},
		{"boundary_min", "1", "1"},
		{"boundary_max", "65535", "65535"},
		{"zero_invalid", "0", "7800"},
		{"over_max", "65536", "7800"},
		{"negative", "-1", "7800"},
		{"non_numeric", "abc", "7800"},
		{"empty_default", "", "7800"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_BIND_ADDR", "DG_POLL_INTERVAL",
				"DG_COMPOSE_PATH", "DG_PASSWORD", "DG_STATS_INTERVAL",
				"DG_STATS_WORKERS")

			t.Setenv("DG_PORT", tt.val)

			cfg := LoadConfig()
			if cfg.Port != tt.want {
				t.Errorf("DG_PORT=%q → got %s, want %s", tt.val, cfg.Port, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DG_POLL_INTERVAL
// ---------------------------------------------------------------------------

func TestLoadConfig_PollInterval(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		wantSecs float64
	}{
		{"valid_10s", "10s", 10},
		{"valid_1m", "1m", 60},
		{"at_minimum_1s", "1s", 1},
		{"below_minimum_clamped", "100ms", 1},
		{"invalid_uses_default", "notaduration", 30},
		{"empty_uses_default", "", 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_COMPOSE_PATH",
				"DG_PASSWORD", "DG_STATS_INTERVAL", "DG_STATS_WORKERS")

			t.Setenv("DG_POLL_INTERVAL", tt.val)

			cfg := LoadConfig()
			if cfg.PollInterval.Seconds() != tt.wantSecs {
				t.Errorf("DG_POLL_INTERVAL=%q → got %v, want %vs",
					tt.val, cfg.PollInterval, tt.wantSecs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DG_STATS_INTERVAL
// ---------------------------------------------------------------------------

func TestLoadConfig_StatsInterval(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		wantSecs float64
	}{
		{"valid_5s", "5s", 5},
		{"valid_10s", "10s", 10},
		{"at_minimum_1s", "1s", 1},
		{"below_minimum_clamped", "500ms", 1},
		{"invalid_uses_default", "garbage", 3},
		{"empty_uses_default", "", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
				"DG_COMPOSE_PATH", "DG_PASSWORD", "DG_STATS_WORKERS")

			t.Setenv("DG_STATS_INTERVAL", tt.val)

			cfg := LoadConfig()
			if cfg.StatsInterval.Seconds() != tt.wantSecs {
				t.Errorf("DG_STATS_INTERVAL=%q → got %v, want %vs",
					tt.val, cfg.StatsInterval, tt.wantSecs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DG_COMPOSE_PATH
// ---------------------------------------------------------------------------

func TestLoadConfig_ComposePaths(t *testing.T) {
	tests := []struct {
		name      string
		val       string
		wantLen   int
		wantPaths []string // nil means check length only
	}{
		{
			"single_path", "/path/to/compose.yml", 1,
			[]string{"/path/to/compose.yml"},
		},
		{
			"multiple_paths", "/a.yml,/b.yml,/c.yml", 3,
			[]string{"/a.yml", "/b.yml", "/c.yml"},
		},
		{
			"whitespace_trimmed", " /a.yml , /b.yml ", 2,
			[]string{"/a.yml", "/b.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
				"DG_PASSWORD", "DG_STATS_INTERVAL", "DG_STATS_WORKERS")

			t.Setenv("DG_COMPOSE_PATH", tt.val)

			cfg := LoadConfig()
			if len(cfg.ComposePaths) != tt.wantLen {
				t.Fatalf("expected %d paths, got %d: %v",
					tt.wantLen, len(cfg.ComposePaths), cfg.ComposePaths)
			}
			for i, want := range tt.wantPaths {
				if cfg.ComposePaths[i] != want {
					t.Errorf("path[%d]: got %q, want %q", i, cfg.ComposePaths[i], want)
				}
			}
		})
	}
}

func TestLoadConfig_ComposePathsEmpty(t *testing.T) {
	clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
		"DG_COMPOSE_PATH", "DG_PASSWORD", "DG_STATS_INTERVAL", "DG_STATS_WORKERS")

	cfg := LoadConfig()
	if cfg.ComposePaths != nil {
		t.Errorf("expected nil ComposePaths when env is empty, got %v", cfg.ComposePaths)
	}
}

// ---------------------------------------------------------------------------
// DG_STATS_WORKERS
// ---------------------------------------------------------------------------

func TestLoadConfig_StatsWorkers(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want int
	}{
		{"valid_10", "10", 10},
		{"valid_100", "100", 100},
		{"zero_falls_back", "0", 50},
		{"negative_falls_back", "-5", 50},
		{"non_numeric_falls_back", "abc", 50},
		{"empty_uses_default", "", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
				"DG_COMPOSE_PATH", "DG_PASSWORD", "DG_STATS_INTERVAL")

			t.Setenv("DG_STATS_WORKERS", tt.val)

			cfg := LoadConfig()
			if cfg.StatsWorkers != tt.want {
				t.Errorf("DG_STATS_WORKERS=%q → got %d, want %d",
					tt.val, cfg.StatsWorkers, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DG_PASSWORD
// ---------------------------------------------------------------------------

func TestLoadConfig_PasswordEmpty(t *testing.T) {
	clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
		"DG_COMPOSE_PATH", "DG_PASSWORD", "DG_STATS_INTERVAL", "DG_STATS_WORKERS")

	cfg := LoadConfig()
	if cfg.PasswordHash != "" {
		t.Errorf("PasswordHash should be empty when DG_PASSWORD is unset, got %q",
			cfg.PasswordHash)
	}
}

func TestLoadConfig_PasswordPlaintext(t *testing.T) {
	clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
		"DG_COMPOSE_PATH", "DG_STATS_INTERVAL", "DG_STATS_WORKERS")

	t.Setenv("DG_PASSWORD", "mysecret")

	cfg := LoadConfig()
	if cfg.PasswordHash == "" {
		t.Fatal("PasswordHash should not be empty")
	}
	if cfg.PasswordHash == "mysecret" {
		t.Error("PasswordHash should be hashed, not stored as plaintext")
	}
	if !auth.IsHashedPassword(cfg.PasswordHash) {
		t.Error("PasswordHash should be a valid argon2id hash")
	}

	// Verify the hash actually matches the original password.
	ok, err := auth.CheckPassword("mysecret", cfg.PasswordHash)
	if err != nil {
		t.Fatalf("CheckPassword error: %v", err)
	}
	if !ok {
		t.Error("hash should verify against the original password")
	}
}

func TestLoadConfig_PasswordPrehashed(t *testing.T) {
	clearConfigEnv(t, "DG_BIND_ADDR", "DG_PORT", "DG_POLL_INTERVAL",
		"DG_COMPOSE_PATH", "DG_STATS_INTERVAL", "DG_STATS_WORKERS")

	hash, err := auth.HashPassword("testpass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	t.Setenv("DG_PASSWORD", hash)

	cfg := LoadConfig()
	if cfg.PasswordHash != hash {
		t.Errorf("pre-hashed password should be stored as-is, got %q", cfg.PasswordHash)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// clearConfigEnv sets the given environment variables to empty strings via
// t.Setenv so they are automatically restored after the test.
func clearConfigEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
	}
}

// ---------------------------------------------------------------------------
// DG_MODE / DG_SWARM_POLL_INTERVAL
// ---------------------------------------------------------------------------

func TestLoadConfig_Mode(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"empty_default", "", "auto"},
		{"auto", "auto", "auto"},
		{"standalone", "standalone", "standalone"},
		{"swarm", "swarm", "swarm"},
		{"agent", "agent", "agent"},
		{"case_insensitive", " Swarm ", "swarm"},
		{"invalid", "cluster", "auto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_PASSWORD")
			t.Setenv("DG_MODE", tt.val)
			if got := LoadConfig().Mode; got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLoadConfig_SwarmPollInterval(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want time.Duration
	}{
		{"default", "", 5 * time.Second},
		{"custom", "10s", 10 * time.Second},
		{"below_min", "100ms", time.Second},
		{"invalid", "soon", 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_PASSWORD")
			t.Setenv("DG_SWARM_POLL_INTERVAL", tt.val)
			if got := LoadConfig().SwarmPollInterval; got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DG_AGENT_PORT / DG_AGENT_ADDR / DG_AGENT_TOKEN(_FILE)
// ---------------------------------------------------------------------------

func TestLoadConfig_AgentPort(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"default", "", "7801"},
		{"custom", "9000", "9000"},
		{"zero", "0", "7801"},
		{"too_high", "70000", "7801"},
		{"not_a_number", "abc", "7801"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_PASSWORD")
			t.Setenv("DG_AGENT_PORT", tt.val)
			if got := LoadConfig().AgentPort; got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLoadConfig_AgentAddr(t *testing.T) {
	clearConfigEnv(t, "DG_PASSWORD")
	t.Setenv("DG_AGENT_ADDR", " tasks.dg_agent:9000 ")
	if got := LoadConfig().AgentAddr; got != "tasks.dg_agent:9000" {
		t.Errorf("got %q, want tasks.dg_agent:9000", got)
	}
}

func TestLoadConfig_AgentToken(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenFile, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(dir, "empty")
	if err := os.WriteFile(emptyFile, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
		file  string
		want  string
	}{
		{"unset", "", "", ""},
		{"inline", " inline-secret ", "", "inline-secret"},
		{"file_trimmed", "", tokenFile, "from-file"},
		{"inline_wins", "inline-secret", tokenFile, "inline-secret"},
		{"missing_file", "", filepath.Join(dir, "missing"), ""},
		{"empty_file", "", emptyFile, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_PASSWORD")
			t.Setenv("DG_AGENT_TOKEN", tt.token)
			t.Setenv("DG_AGENT_TOKEN_FILE", tt.file)
			if got := LoadConfig().AgentToken; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DG_PASSWORD_FILE
// ---------------------------------------------------------------------------

func TestLoadConfig_PasswordFile(t *testing.T) {
	dir := t.TempDir()
	plainFile := filepath.Join(dir, "plain")
	if err := os.WriteFile(plainFile, []byte("from-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("hashed-secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	hashFile := filepath.Join(dir, "hash")
	if err := os.WriteFile(hashFile, []byte(hash+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		inline string
		file   string
		want   string // password expected to verify; "" = auth disabled
	}{
		{"unset", "", "", ""},
		{"file_plaintext_trimmed", "", plainFile, "from-secret"},
		{"file_prehashed", "", hashFile, "hashed-secret"},
		{"inline_wins", "inline-pass", plainFile, "inline-pass"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_AGENT_TOKEN", "DG_AGENT_TOKEN_FILE")
			t.Setenv("DG_PASSWORD", tt.inline)
			t.Setenv("DG_PASSWORD_FILE", tt.file)
			got := LoadConfig().PasswordHash
			if tt.want == "" {
				if got != "" {
					t.Fatalf("PasswordHash: got %q, want empty", got)
				}
				return
			}
			ok, err := auth.CheckPassword(tt.want, got)
			if err != nil {
				t.Fatalf("CheckPassword: %v", err)
			}
			if !ok {
				t.Errorf("hash does not verify against %q", tt.want)
			}
		})
	}
}

func TestReadSecretEnv_MissingFile(t *testing.T) {
	t.Setenv("DG_TEST_SECRET", "")
	t.Setenv("DG_TEST_SECRET_FILE", filepath.Join(t.TempDir(), "missing"))
	if _, err := readSecretEnv("DG_TEST_SECRET"); err == nil {
		t.Fatal("expected an error for an unreadable secret file")
	}
}

func TestReadSecretEnv_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"empty": "", "whitespace": " \n\t\n"} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(dir, name)
			if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DG_TEST_SECRET", "")
			t.Setenv("DG_TEST_SECRET_FILE", file)
			_, err := readSecretEnv("DG_TEST_SECRET")
			if err == nil || !strings.Contains(err.Error(), "DG_TEST_SECRET_FILE is empty") {
				t.Fatalf("expected an empty-file error, got %v", err)
			}
		})
	}
}

// The inline variable wins before the file is read, so an empty file next
// to it is not an error.
func TestReadSecretEnv_InlineWinsOverEmptyFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DG_TEST_SECRET", "inline")
	t.Setenv("DG_TEST_SECRET_FILE", file)
	got, err := readSecretEnv("DG_TEST_SECRET")
	if err != nil || got != "inline" {
		t.Fatalf("got %q, %v; want inline, nil", got, err)
	}
}

func TestHealthcheckPort(t *testing.T) {
	tests := []struct {
		name, mode, port, agentPort, want string
	}{
		{"server_default", "", "", "", "7800"},
		{"server_port", "swarm", "9000", "9001", "9000"},
		{"agent_default", "agent", "9000", "", "7801"},
		{"agent_port", "agent", "9000", "9001", "9001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t, "DG_PASSWORD")
			t.Setenv("DG_MODE", tt.mode)
			t.Setenv("DG_PORT", tt.port)
			t.Setenv("DG_AGENT_PORT", tt.agentPort)
			// An unreadable password file would make LoadConfig exit: the
			// probe must not read it (nor hash the password) at all.
			t.Setenv("DG_PASSWORD_FILE", filepath.Join(t.TempDir(), "missing"))
			if got := healthcheckPort(); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}
