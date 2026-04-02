package api

import (
	"testing"
)

func TestValidResourceName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid names
		{"alphanumeric", "mycontainer1", true},
		{"with dots", "my.container", true},
		{"with dashes", "my-container", true},
		{"with underscores", "my_container", true},
		{"mixed separators", "my-app_v2.1", true},
		{"all digits", "1234567890", true},
		{"uppercase letters", "MyContainer", true},
		{"long name", "abcdefghijklmnopqrstuvwxyz0123456789", true},
		{"digit start with dot", "1.0", true},

		// Invalid names
		{"empty string", "", false},
		{"single char", "a", false},
		{"starts with dot", ".hidden", false},
		{"starts with dash", "-invalid", false},
		{"starts with underscore", "_private", false},
		{"contains spaces", "my container", false},
		{"contains slash", "my/container", false},
		{"contains colon", "my:container", false},
		{"contains at sign", "user@host", false},
		{"contains exclamation", "danger!", false},
		{"contains hash", "issue#42", false},
		{"only special chars", "!@#$%", false},
		{"newline in name", "my\ncontainer", false},
		{"tab in name", "my\tcontainer", false},
		{"unicode chars", "containe\u00e9r", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validResourceName.MatchString(tc.input)
			if got != tc.want {
				t.Errorf("validResourceName.MatchString(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		// Each pattern in sensitiveKeyPatterns
		{"PASSWORD", "DB_PASSWORD", true},
		{"SECRET", "JWT_SECRET", true},
		{"KEY", "SIGNING_KEY", true},
		{"TOKEN", "ACCESS_TOKEN", true},
		{"CREDENTIAL", "AWS_CREDENTIAL", true},
		{"API_KEY", "STRIPE_API_KEY", true},
		{"APIKEY", "STRIPE_APIKEY", true},
		{"PRIVATE", "PRIVATE_KEY_PATH", true},
		{"AUTH", "AUTH_HEADER", true},
		{"CERT", "TLS_CERT_FILE", true},
		{"SSL_ prefix", "SSL_CERT_PATH", true},
		{"TLS_ prefix", "TLS_KEY_FILE", true},
		{"ENCRYPT", "ENCRYPT_ALGO", true},
		{"ACCESS_KEY", "AWS_ACCESS_KEY_ID", true},
		{"SESSION", "SESSION_SECRET", true},

		// Case insensitivity
		{"lowercase password", "db_password", true},
		{"mixed case secret", "Jwt_Secret", true},
		{"all lowercase token", "access_token", true},
		{"camelCase auth", "oAuthToken", true},

		// Non-sensitive keys
		{"PATH", "PATH", false},
		{"HOME", "HOME", false},
		{"PORT", "PORT", false},
		{"DATABASE_URL", "DATABASE_URL", false},
		{"LOG_LEVEL", "LOG_LEVEL", false},
		{"NODE_ENV", "NODE_ENV", false},
		{"HOSTNAME", "HOSTNAME", false},
		{"empty key", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isSensitiveKey(tc.key)
			if got != tc.want {
				t.Errorf("isSensitiveKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestFilterEnvVars(t *testing.T) {
	t.Run("normal vars", func(t *testing.T) {
		input := []string{"PATH=/usr/bin", "HOME=/root", "PORT=8080"}
		result := filterEnvVars(input)

		if len(result) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(result))
		}

		for _, entry := range result {
			if entry["value"] == "********" {
				t.Errorf("non-sensitive var %q should not be masked", entry["key"])
			}
		}

		// Verify specific values preserved
		found := false
		for _, entry := range result {
			if entry["key"] == "PATH" && entry["value"] == "/usr/bin" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected PATH=/usr/bin to be present and unmasked")
		}
	})

	t.Run("sensitive vars masked", func(t *testing.T) {
		sensitive := []string{
			"DB_PASSWORD=supersecret",
			"JWT_SECRET=mysecret",
			"API_KEY=abc123",
			"ACCESS_TOKEN=tok-xyz",
			"AWS_CREDENTIAL=cred",
			"STRIPE_APIKEY=sk_live",
			"PRIVATE_KEY=rsa-key",
			"AUTH_HEADER=bearer",
			"SSL_CERT=cert-data",
			"TLS_KEY=key-data",
			"ENCRYPT_KEY=enc",
			"AWS_ACCESS_KEY_ID=AKIA",
			"SESSION_ID=sess123",
		}

		result := filterEnvVars(sensitive)
		for _, entry := range result {
			if entry["value"] != "********" {
				t.Errorf("sensitive var %q should be masked, got value %q", entry["key"], entry["value"])
			}
		}
	})

	t.Run("case insensitive masking", func(t *testing.T) {
		input := []string{"db_password=secret", "Jwt_Secret=val"}
		result := filterEnvVars(input)

		for _, entry := range result {
			if entry["value"] != "********" {
				t.Errorf("sensitive var %q (case-insensitive) should be masked, got %q", entry["key"], entry["value"])
			}
		}
	})

	t.Run("var without equals sign", func(t *testing.T) {
		input := []string{"STANDALONE_VAR"}
		result := filterEnvVars(input)

		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["key"] != "STANDALONE_VAR" {
			t.Errorf("expected key STANDALONE_VAR, got %q", result[0]["key"])
		}
		if result[0]["value"] != "" {
			t.Errorf("expected empty value for var without '=', got %q", result[0]["value"])
		}
	})

	t.Run("empty list", func(t *testing.T) {
		result := filterEnvVars([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d entries", len(result))
		}
	})

	t.Run("value-less var with equals", func(t *testing.T) {
		input := []string{"EMPTY_VAL="}
		result := filterEnvVars(input)

		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["key"] != "EMPTY_VAL" {
			t.Errorf("expected key EMPTY_VAL, got %q", result[0]["key"])
		}
		if result[0]["value"] != "" {
			t.Errorf("expected empty value, got %q", result[0]["value"])
		}
	})

	t.Run("sensitive var without equals", func(t *testing.T) {
		input := []string{"MY_PASSWORD"}
		result := filterEnvVars(input)

		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["value"] != "********" {
			t.Errorf("sensitive var without '=' should still be masked, got %q", result[0]["value"])
		}
	})

	t.Run("value containing equals sign", func(t *testing.T) {
		input := []string{"CONFIG=key=value=extra"}
		result := filterEnvVars(input)

		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["key"] != "CONFIG" {
			t.Errorf("expected key CONFIG, got %q", result[0]["key"])
		}
		if result[0]["value"] != "key=value=extra" {
			t.Errorf("expected value to preserve inner '=' signs, got %q", result[0]["value"])
		}
	})
}
