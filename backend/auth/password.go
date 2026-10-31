package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
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

// Bounds on the parameters of a configured (pre-hashed) password. A check
// allocates the hash's memory cost and holds the single check slot (see
// maxConcurrentChecks) for all its passes, so a hash outside these bounds is
// refused at startup. Short salts and keys mean a broken hash.
const (
	minHashSaltLen = 8
	minHashKeyLen  = 16
	maxHashMemory  = 128 * 1024 // KiB: twice the default, within the shipped 256M limit
	// maxHashTime covers the usual profiles of 1 to 5 passes, while a check
	// at maxHashMemory stays well within loginQueueTimeout.
	maxHashTime = 10
)

// maxConcurrentChecks bounds the password verifications running at once,
// process-wide, since each holds the hash's memory cost (64 MiB by default).
// Logins are rare and human-paced, so serialising them costs a real user
// nothing and keeps the memory logins take bounded.
const maxConcurrentChecks = 1

var checkSlots = make(chan struct{}, maxConcurrentChecks)

// ErrBusy is returned by CheckPasswordBounded when no verification slot
// freed up before its context ended.
var ErrBusy = errors.New("too many concurrent password checks")

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
	h, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	actualKey := argon2.IDKey([]byte(password), h.salt, h.time, h.memory, h.threads, uint32(len(h.key)))
	return subtle.ConstantTimeCompare(actualKey, h.key) == 1, nil
}

// CheckPasswordBounded is CheckPassword limited to maxConcurrentChecks
// verifications at a time. It waits for a free slot until ctx ends, then
// gives up with ErrBusy.
func CheckPasswordBounded(ctx context.Context, password, encoded string) (bool, error) {
	select {
	case checkSlots <- struct{}{}:
	case <-ctx.Done():
		return false, ErrBusy
	}
	defer func() { <-checkSlots }()
	return CheckPassword(password, encoded)
}

// IsHashedPassword checks if a string looks like a pre-hashed Argon2id value.
func IsHashedPassword(s string) bool {
	return strings.HasPrefix(s, "$argon2id$")
}

// ValidateHash checks if a hash string is well-formed.
func ValidateHash(encoded string) error {
	_, err := decodeHash(encoded)
	return err
}

func encodeHash(salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// argon2Hash is a decoded Argon2id hash: its salt, key and cost parameters.
type argon2Hash struct {
	salt, key []byte
	time      uint32
	memory    uint32 // KiB
	threads   uint8
}

func decodeHash(encoded string) (argon2Hash, error) {
	var h argon2Hash
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argon2Hash{}, errors.New("invalid argon2id hash format")
	}
	version, err := parseParam(parts[2], "v", 32)
	if err != nil {
		return argon2Hash{}, fmt.Errorf("parse version: %w", err)
	}
	if version != argon2.Version {
		return argon2Hash{}, fmt.Errorf("unsupported argon2 version %d (want %d)", version, argon2.Version)
	}
	h.memory, h.time, h.threads, err = parseParams(parts[3])
	if err != nil {
		return argon2Hash{}, fmt.Errorf("parse params: %w", err)
	}
	if h.time < 1 || h.time > maxHashTime {
		return argon2Hash{}, fmt.Errorf("invalid params: t must be between 1 and %d", maxHashTime)
	}
	if h.threads < 1 {
		return argon2Hash{}, errors.New("invalid params: p must be at least 1")
	}
	if h.memory < 8*uint32(h.threads) || h.memory > maxHashMemory {
		return argon2Hash{}, fmt.Errorf("invalid params: m must be between 8*p and %d KiB", maxHashMemory)
	}
	h.salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Hash{}, fmt.Errorf("decode salt: %w", err)
	}
	if len(h.salt) < minHashSaltLen {
		return argon2Hash{}, fmt.Errorf("salt too short: %d bytes (want at least %d)", len(h.salt), minHashSaltLen)
	}
	h.key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2Hash{}, fmt.Errorf("decode key: %w", err)
	}
	if len(h.key) < minHashKeyLen {
		return argon2Hash{}, fmt.Errorf("key too short: %d bytes (want at least %d)", len(h.key), minHashKeyLen)
	}
	return h, nil
}

// parseParams parses the "m=65536,t=1,p=4" field of an encoded hash. It is
// strict, exactly these three in this order as plain decimal numbers, so a
// field that another tool would read differently is refused rather than
// read in part.
func parseParams(field string) (memory, time uint32, threads uint8, err error) {
	params := strings.Split(field, ",")
	if len(params) != 3 {
		return 0, 0, 0, fmt.Errorf("want m=<KiB>,t=<passes>,p=<lanes>, got %q", field)
	}
	m, err := parseParam(params[0], "m", 32)
	if err != nil {
		return 0, 0, 0, err
	}
	t, err := parseParam(params[1], "t", 32)
	if err != nil {
		return 0, 0, 0, err
	}
	p, err := parseParam(params[2], "p", 8)
	if err != nil {
		return 0, 0, 0, err
	}
	return uint32(m), uint32(t), uint8(p), nil
}

// parseParam parses one "name=value" parameter whose decimal value fits in
// bits.
func parseParam(param, name string, bits int) (uint64, error) {
	value, ok := strings.CutPrefix(param, name+"=")
	if !ok {
		return 0, fmt.Errorf("want %s=<number>, got %q", name, param)
	}
	n, err := strconv.ParseUint(value, 10, bits)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return n, nil
}
