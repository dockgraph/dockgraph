package secrets

import "testing"

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
			got := IsSensitiveKey(tc.key)
			if got != tc.want {
				t.Errorf("IsSensitiveKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}
