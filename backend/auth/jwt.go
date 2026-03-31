package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claims represents the JWT payload.
type Claims struct {
	IssuedAt  time.Time
	ExpiresAt time.Time
	JTI       string
}

// GenerateSigningKey creates a random 32-byte HMAC signing key.
func GenerateSigningKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	return key, nil
}

// GenerateJTI creates a random token identifier.
func GenerateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

var jwtHeader = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// SignToken creates an HS256 JWT with the given claims.
func SignToken(c Claims, key []byte) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"iat": c.IssuedAt.Unix(),
		"exp": c.ExpiresAt.Unix(),
		"jti": c.JTI,
	})
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	body := jwtHeader + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

// VerifyToken validates an HS256 JWT and returns its claims.
func VerifyToken(token string, key []byte) (Claims, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("invalid token format")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("decode header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return Claims{}, fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "HS256" {
		return Claims{}, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := mac.Sum(nil)
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, fmt.Errorf("decode signature: %w", err)
	}
	if !hmac.Equal(expectedSig, actualSig) {
		return Claims{}, fmt.Errorf("invalid signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("decode payload: %w", err)
	}
	var raw struct {
		IAT float64 `json:"iat"`
		EXP float64 `json:"exp"`
		JTI string  `json:"jti"`
	}
	if err := json.Unmarshal(payloadJSON, &raw); err != nil {
		return Claims{}, fmt.Errorf("parse claims: %w", err)
	}

	claims := Claims{
		IssuedAt:  time.Unix(int64(raw.IAT), 0),
		ExpiresAt: time.Unix(int64(raw.EXP), 0),
		JTI:       raw.JTI,
	}
	if time.Now().After(claims.ExpiresAt) {
		return Claims{}, fmt.Errorf("token expired")
	}
	return claims, nil
}
