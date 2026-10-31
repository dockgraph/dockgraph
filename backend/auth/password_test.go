package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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

const (
	testHashSalt = "c29tZXNhbHRzb21lc2FsdA"                      // 16 bytes
	testHashKey  = "a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2U" // 32 bytes
)

// Parameters argon2 cannot run with (it panics on t=0 or p=0), memory and
// time costs the server cannot afford on every login, malformed parameter
// fields, and empty or short salts and keys must be refused at startup
// rather than fail, crash, stall or match anything at login.
func TestValidateHashRejectsUnsafeParams(t *testing.T) {
	salt, key := testHashSalt, testHashKey
	for _, tc := range []struct {
		name, hash string
	}{
		{"zero rounds", "$argon2id$v=19$m=65536,t=0,p=4$" + salt + "$" + key},
		{"too many rounds", "$argon2id$v=19$m=65536,t=11,p=4$" + salt + "$" + key},
		{"largest round count", "$argon2id$v=19$m=65536,t=4294967295,p=4$" + salt + "$" + key},
		{"zero parallelism", "$argon2id$v=19$m=65536,t=1,p=0$" + salt + "$" + key},
		{"parallelism above 255", "$argon2id$v=19$m=65536,t=1,p=256$" + salt + "$" + key},
		{"memory below 8*p", "$argon2id$v=19$m=16,t=1,p=4$" + salt + "$" + key},
		{"memory too high", "$argon2id$v=19$m=4194304,t=1,p=4$" + salt + "$" + key},
		{"trailing junk in params", "$argon2id$v=19$m=65536,t=1,p=4,junk$" + salt + "$" + key},
		{"params out of order", "$argon2id$v=19$t=1,m=65536,p=4$" + salt + "$" + key},
		{"missing param", "$argon2id$v=19$m=65536,t=1$" + salt + "$" + key},
		{"signed param", "$argon2id$v=19$m=+65536,t=1,p=4$" + salt + "$" + key},
		{"space in params", "$argon2id$v=19$m= 65536,t=1,p=4$" + salt + "$" + key},
		{"wrong version", "$argon2id$v=16$m=65536,t=1,p=4$" + salt + "$" + key},
		{"trailing junk in version", "$argon2id$v=19x$m=65536,t=1,p=4$" + salt + "$" + key},
		{"empty key", "$argon2id$v=19$m=65536,t=1,p=4$" + salt + "$"},
		{"short salt", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$" + key},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateHash(tc.hash); err == nil {
				t.Errorf("ValidateHash(%q) succeeded, want an error", tc.hash)
			}
			if ok, err := CheckPassword("anything", tc.hash); ok || err == nil {
				t.Errorf("CheckPassword = %v, %v; want false and an error", ok, err)
			}
		})
	}
}

// Hashes made with common tools and guides must keep working: the bounds
// only refuse costs that no usual profile reaches.
func TestValidateHashAcceptsCommonProfiles(t *testing.T) {
	for _, tc := range []struct {
		name, params string
	}{
		{"DockGraph's own", "m=65536,t=1,p=4"},
		{"OWASP", "m=19456,t=2,p=1"},
		{"RFC 9106 for constrained memory", "m=65536,t=3,p=4"},
		{"argon2 command-line defaults", "m=4096,t=3,p=1"},
		{"PHP password_hash defaults", "m=65536,t=4,p=1"},
		{"largest allowed cost", "m=131072,t=10,p=4"},
	} {
		hash := "$argon2id$v=19$" + tc.params + "$" + testHashSalt + "$" + testHashKey
		if err := ValidateHash(hash); err != nil {
			t.Errorf("%s profile (%s) rejected: %v", tc.name, tc.params, err)
		}
	}
}

func TestCheckPasswordBoundedGivesUpWhenBusy(t *testing.T) {
	hash, _ := HashPassword("correct")
	holdCheckSlots(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := CheckPasswordBounded(ctx, "correct", hash); !errors.Is(err, ErrBusy) {
		t.Errorf("err = %v, want ErrBusy", err)
	}
}

func TestCheckPasswordBoundedReleasesSlot(t *testing.T) {
	hash, _ := HashPassword("correct")
	for i := range maxConcurrentChecks + 2 {
		ok, err := CheckPasswordBounded(context.Background(), "correct", hash)
		if err != nil || !ok {
			t.Fatalf("check %d = %v, %v; want true", i, ok, err)
		}
	}
}
