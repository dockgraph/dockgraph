package main

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	os.Unsetenv("DF_PORT")
	os.Unsetenv("DF_POLL_INTERVAL")
	os.Unsetenv("DF_COMPOSE_DIR")

	cfg := LoadConfig()

	if cfg.Port != "7800" {
		t.Errorf("expected default port 7800, got %s", cfg.Port)
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
				os.Unsetenv("DF_PORT")
			} else {
				t.Setenv("DF_PORT", tt.val)
			}

			cfg := LoadConfig()
			if cfg.Port != tt.want {
				t.Errorf("DF_PORT=%q: got %s, want %s", tt.val, cfg.Port, tt.want)
			}
		})
	}
}
