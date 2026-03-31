package auth

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestGenerateSigningKeyLength(t *testing.T) {
	key, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}
}

func TestGenerateSigningKeyUnique(t *testing.T) {
	k1, _ := GenerateSigningKey()
	k2, _ := GenerateSigningKey()
	if string(k1) == string(k2) {
		t.Error("two keys should differ")
	}
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	key, _ := GenerateSigningKey()
	now := time.Now()
	original := Claims{
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
		JTI:       "test-jti-123",
	}
	token, err := SignToken(original, key)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	got, err := VerifyToken(token, key)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if got.JTI != original.JTI {
		t.Errorf("JTI = %q, want %q", got.JTI, original.JTI)
	}
	if got.IssuedAt.Unix() != now.Unix() {
		t.Errorf("IssuedAt = %d, want %d", got.IssuedAt.Unix(), now.Unix())
	}
	if got.ExpiresAt.Unix() != original.ExpiresAt.Unix() {
		t.Errorf("ExpiresAt = %d, want %d", got.ExpiresAt.Unix(), original.ExpiresAt.Unix())
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	key, _ := GenerateSigningKey()
	past := time.Now().Add(-2 * time.Hour)
	token, _ := SignToken(Claims{IssuedAt: past, ExpiresAt: past.Add(time.Hour), JTI: "x"}, key)
	_, err := VerifyToken(token, key)
	if err == nil {
		t.Error("expired token should fail")
	}
}

func TestVerifyTokenWrongKey(t *testing.T) {
	k1, _ := GenerateSigningKey()
	k2, _ := GenerateSigningKey()
	token, _ := SignToken(Claims{IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), JTI: "x"}, k1)
	_, err := VerifyToken(token, k2)
	if err == nil {
		t.Error("wrong key should fail")
	}
}

func TestVerifyTokenMalformed(t *testing.T) {
	key, _ := GenerateSigningKey()
	for _, token := range []string{"", "a.b", "a.b.c.d", "not-a-jwt"} {
		if _, err := VerifyToken(token, key); err == nil {
			t.Errorf("malformed token %q should fail", token)
		}
	}
}

func TestVerifyTokenRejectsNoneAlgorithm(t *testing.T) {
	key, _ := GenerateSigningKey()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(fmt.Sprintf(`{"iat":%d,"exp":%d,"jti":"x"}`, time.Now().Unix(), time.Now().Add(time.Hour).Unix())),
	)
	token := header + "." + payload + "."
	_, err := VerifyToken(token, key)
	if err == nil {
		t.Error("'none' algorithm should be rejected")
	}
}

func TestGenerateJTI(t *testing.T) {
	jti, err := GenerateJTI()
	if err != nil {
		t.Fatalf("GenerateJTI: %v", err)
	}
	if len(jti) == 0 {
		t.Error("JTI should not be empty")
	}
}

func TestGenerateJTIUnique(t *testing.T) {
	j1, _ := GenerateJTI()
	j2, _ := GenerateJTI()
	if j1 == j2 {
		t.Error("two JTIs should differ")
	}
}
