package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2SaltLen = 16
	argon2KeyLen  = 32
)

// HashPassword creates an Argon2id hash of the given password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return encodeHash(salt, key), nil
}

// CheckPassword verifies a password against an Argon2id hash.
func CheckPassword(password, encoded string) (bool, error) {
	salt, expectedKey, time, memory, threads, keyLen, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	actualKey := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	return subtle.ConstantTimeCompare(actualKey, expectedKey) == 1, nil
}

// IsHashedPassword checks if a string looks like a pre-hashed Argon2id value.
func IsHashedPassword(s string) bool {
	return strings.HasPrefix(s, "$argon2id$")
}

// ValidateHash checks if a hash string is well-formed.
func ValidateHash(encoded string) error {
	_, _, _, _, _, _, err := decodeHash(encoded)
	return err
}

func encodeHash(salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

func decodeHash(encoded string) (salt, key []byte, time, memory uint32, threads uint8, keyLen uint32, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, 0, fmt.Errorf("invalid argon2id hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, 0, 0, 0, 0, fmt.Errorf("parse version: %w", err)
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return nil, nil, 0, 0, 0, 0, fmt.Errorf("parse params: %w", err)
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, 0, 0, 0, 0, fmt.Errorf("decode salt: %w", err)
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, 0, 0, 0, 0, fmt.Errorf("decode key: %w", err)
	}
	return salt, key, time, memory, threads, uint32(len(key)), nil
}
