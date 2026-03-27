package main

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	os.Unsetenv("DG_BIND_ADDR")
	os.Unsetenv("DG_PORT")
	os.Unsetenv("DG_POLL_INTERVAL")
	os.Unsetenv("DG_COMPOSE_PATH")

	cfg := LoadConfig()

	if cfg.BindAddr != "0.0.0.0" {
		t.Errorf("expected default bind addr 0.0.0.0, got %s", cfg.BindAddr)
	}
	if cfg.Port != "7800" {
		t.Errorf("expected default port 7800, got %s", cfg.Port)
	}
	if cfg.ComposePaths != nil {
		t.Errorf("expected nil ComposePaths for auto-detect, got %v", cfg.ComposePaths)
	}
}

func TestLoadConfigBindAddr(t *testing.T) {
	t.Setenv("DG_BIND_ADDR", "127.0.0.1")
	os.Unsetenv("DG_PORT")
	os.Unsetenv("DG_POLL_INTERVAL")
	os.Unsetenv("DG_COMPOSE_PATH")

	cfg := LoadConfig()
	if cfg.BindAddr != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", cfg.BindAddr)
	}
}

func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"letters", "abc", "7800"},
		{"zero", "0", "7800"},
		{"negative", "-1", "7800"},
		{"too_high", "70000", "7800"},
		{"boundary_max", "65535", "65535"},
		{"over_max", "65536", "7800"},
		{"boundary_min", "1", "1"},
		{"empty", "", "7800"},
		{"valid", "8080", "8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val == "" {
				os.Unsetenv("DG_PORT")
			} else {
				t.Setenv("DG_PORT", tt.val)
			}

			cfg := LoadConfig()
			if cfg.Port != tt.want {
				t.Errorf("DG_PORT=%q: got %s, want %s", tt.val, cfg.Port, tt.want)
			}
		})
	}
}

func TestLoadConfigPollInterval(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		wantSecs float64
	}{
		{"valid_10s", "10s", 10},
		{"valid_1m", "1m", 60},
		{"too_low_clamped", "100ms", 1},
		{"boundary_1s", "1s", 1},
		{"invalid_ignored", "notaduration", 30},
		{"empty_default", "", 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val == "" {
				os.Unsetenv("DG_POLL_INTERVAL")
			} else {
				t.Setenv("DG_POLL_INTERVAL", tt.val)
			}
			os.Unsetenv("DG_PORT")
			os.Unsetenv("DG_COMPOSE_PATH")

			cfg := LoadConfig()
			if cfg.PollInterval.Seconds() != tt.wantSecs {
				t.Errorf("DG_POLL_INTERVAL=%q: got %v, want %vs", tt.val, cfg.PollInterval, tt.wantSecs)
			}
		})
	}
}

func TestLoadConfigComposePaths(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want int
	}{
		{"single", "/path/to/compose.yml", 1},
		{"multiple", "/a.yml,/b.yml,/c.yml", 3},
		{"empty_default", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val == "" {
				os.Unsetenv("DG_COMPOSE_PATH")
			} else {
				t.Setenv("DG_COMPOSE_PATH", tt.val)
			}
			os.Unsetenv("DG_PORT")
			os.Unsetenv("DG_POLL_INTERVAL")

			cfg := LoadConfig()
			if tt.want == 0 {
				if cfg.ComposePaths != nil {
					t.Errorf("expected nil ComposePaths, got %v", cfg.ComposePaths)
				}
			} else if len(cfg.ComposePaths) != tt.want {
				t.Errorf("expected %d paths, got %d", tt.want, len(cfg.ComposePaths))
			}
		})
	}
}
