package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordFormat(t *testing.T) {
	hash, err := HashPassword("testpass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash should start with $argon2id$, got %s", hash)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Errorf("hash should have 6 $-delimited parts, got %d", len(parts))
	}
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of the same password should differ (different salts)")
	}
}

func TestCheckPasswordCorrect(t *testing.T) {
	hash, _ := HashPassword("correct")
	ok, err := CheckPassword("correct", hash)
	if err != nil {
		t.Fatalf("CheckPassword: %v", err)
	}
	if !ok {
		t.Error("correct password should match")
	}
}

func TestCheckPasswordWrong(t *testing.T) {
	hash, _ := HashPassword("correct")
	ok, err := CheckPassword("wrong", hash)
	if err != nil {
		t.Fatalf("CheckPassword: %v", err)
	}
	if ok {
		t.Error("wrong password should not match")
	}
}

func TestCheckPasswordMalformedHash(t *testing.T) {
	_, err := CheckPassword("pass", "not-a-hash")
	if err == nil {
		t.Error("malformed hash should return error")
	}
}

func TestIsHashedPassword(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"$argon2id$v=19$m=65536,t=1,p=4$salt$key", true},
		{"plaintext", false},
		{"$2a$10$bcrypthash", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsHashedPassword(tt.input); got != tt.want {
			t.Errorf("IsHashedPassword(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestValidateHashValid(t *testing.T) {
	hash, _ := HashPassword("test")
	if err := ValidateHash(hash); err != nil {
		t.Errorf("valid hash should pass: %v", err)
	}
}

func TestValidateHashInvalid(t *testing.T) {
	if err := ValidateHash("$argon2id$garbage"); err == nil {
		t.Error("invalid hash should fail")
	}
}
