// Package agentauth authenticates the swarm server to its per-node agents.
//
// The server holds an Ed25519 private key and signs every request it sends
// to an agent; agents hold only the public key. An agent therefore accepts
// requests from the server alone: a key that lets a node check signatures
// does not let it make them. A signature covers the method, the host the
// request is addressed to, the path and query, a timestamp and a random
// nonce, so it cannot be moved to another request, sent to another agent,
// or replayed.
//
// The shared bearer token (Token) is the older scheme, kept so existing
// deployments keep working while they move to keys.
package agentauth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Text forms of the keys. The prefixes keep the private key from being
// mistaken for the public one (agents must never be given the private key)
// and carry the format version.
const (
	privateKeyPrefix = "dgagent-private-v1:"
	publicKeyPrefix  = "dgagent-public-v1:"
)

// SignatureHeader carries a request's signature. It is kept apart from
// Authorization so a server moving from the token to keys can send both,
// and an agent of either kind finds the credential it checks.
const SignatureHeader = "Dockgraph-Signature"

// MaxClockSkew is how far a signed request's timestamp may be from the
// agent's clock, in either direction.
const MaxClockSkew = 2 * time.Minute

// signingContext opens every signed payload, so a signature made for an
// agent request can never pass for anything else signed with the key.
const signingContext = "dockgraph-agent-request-v1"

const nonceSize = 16

var b64 = base64.RawURLEncoding

// GenerateKey returns a new private key in its text form.
func GenerateKey() (string, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate agent key: %w", err)
	}
	return privateKeyPrefix + b64.EncodeToString(key.Seed()), nil
}

// ParsePrivateKey parses a private key in its text form (surrounding
// whitespace, such as a trailing newline, is ignored).
func ParsePrivateKey(s string) (ed25519.PrivateKey, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, publicKeyPrefix) {
		return nil, errors.New("this is an agent public key; the server needs the private key from --agent-keygen")
	}
	enc, ok := strings.CutPrefix(s, privateKeyPrefix)
	if !ok {
		return nil, fmt.Errorf("not an agent private key (it should start with %q)", privateKeyPrefix)
	}
	seed, err := b64.DecodeString(enc)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("malformed agent private key")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// PublicKeyText returns the text form of key's public key.
func PublicKeyText(key ed25519.PrivateKey) string {
	return publicKeyPrefix + b64.EncodeToString(key.Public().(ed25519.PublicKey))
}

// ParsePublicKeys parses one or more public keys separated by whitespace or
// commas. Listing two lets agents accept a new server key before the old one
// is retired.
func ParsePublicKeys(s string) ([]ed25519.PublicKey, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	keys := make([]ed25519.PublicKey, 0, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f, privateKeyPrefix) {
			return nil, errors.New("this is the server's private key: agents take only the public key (--agent-pubkey)")
		}
		enc, ok := strings.CutPrefix(f, publicKeyPrefix)
		if !ok {
			return nil, fmt.Errorf("not an agent public key (it should start with %q)", publicKeyPrefix)
		}
		raw, err := b64.DecodeString(enc)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, errors.New("malformed agent public key")
		}
		keys = append(keys, ed25519.PublicKey(raw))
	}
	if len(keys) == 0 {
		return nil, errors.New("no agent public key found")
	}
	return keys, nil
}

// Authorizer adds the server's credentials to a request for an agent. Call
// it once the request's method, host and URL are final: a signature covers
// them.
type Authorizer interface {
	Authorize(r *http.Request) error
}

// Token authorizes requests with the shared bearer token.
type Token string

// Authorize sets the Authorization header.
func (t Token) Authorize(r *http.Request) error {
	r.Header.Set("Authorization", "Bearer "+string(t))
	return nil
}

// Signer authorizes requests by signing them with the server's private key.
type Signer struct {
	key ed25519.PrivateKey
	now func() time.Time
}

// NewSigner returns a Signer for key.
func NewSigner(key ed25519.PrivateKey) *Signer {
	return &Signer{key: key, now: time.Now}
}

// Authorize sets SignatureHeader.
func (s *Signer) Authorize(r *http.Request) error {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("sign agent request: %w", err)
	}
	ts := s.now().Unix()
	n := b64.EncodeToString(nonce)
	sig := ed25519.Sign(s.key, payload(r.Method, requestHost(r), r.URL.RequestURI(), ts, n))
	r.Header.Set(SignatureHeader, fmt.Sprintf("t=%d, n=%s, s=%s", ts, n, b64.EncodeToString(sig)))
	return nil
}

// Chain applies several authorizers in turn: a server moving to keys signs
// its requests and still sends the token to agents not switched yet.
type Chain []Authorizer

// Authorize applies every authorizer of the chain.
func (c Chain) Authorize(r *http.Request) error {
	for _, a := range c {
		if err := a.Authorize(r); err != nil {
			return err
		}
	}
	return nil
}

// requestHost returns the host an outgoing request is addressed to, as the
// HTTP client sends it in the Host header.
func requestHost(r *http.Request) string {
	if r.Host != "" {
		return r.Host
	}
	return r.URL.Host
}

// payload returns the bytes a signature covers. No field can contain a
// newline: HTTP rejects one in the method, host and request target, and the
// timestamp and nonce are checked before use.
func payload(method, host, requestURI string, ts int64, nonce string) []byte {
	var b bytes.Buffer
	for _, part := range []string{signingContext, method, host, requestURI, strconv.FormatInt(ts, 10), nonce} {
		b.WriteString(part)
		b.WriteByte('\n')
	}
	return b.Bytes()
}
