package agentauth

import (
	"crypto/ed25519"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Reasons Verify rejects a request, so an agent can log what went wrong.
var (
	ErrUnsigned     = errors.New("request is not signed")
	ErrMalformed    = errors.New("malformed signature")
	ErrStale        = errors.New("signature timestamp outside the allowed clock skew")
	ErrBadSignature = errors.New("signature does not match any configured public key")
	ErrWrongHost    = errors.New("request was signed for another host")
	ErrReplayed     = errors.New("request was already received")
)

// The replay cache holds each nonce while its timestamp is acceptable.
// seenPruneSize is the size at which expired nonces are first dropped (see
// remember), and maxSeenNonces bounds the cache whatever happens.
const (
	seenPruneSize = 1 << 10
	maxSeenNonces = 1 << 16
)

// localAddrRefresh is how often, at most, a request addressed to an address
// the agent doesn't know makes it re-read its interface addresses (a network
// may have been attached since).
const localAddrRefresh = time.Second

// Verifier checks the signed requests an agent receives.
type Verifier struct {
	keys  []ed25519.PublicKey
	now   func() time.Time
	addrs func() ([]net.Addr, error)

	mu        sync.Mutex
	seen      map[string]time.Time // nonce → time it stops being acceptable
	pruneAt   int                  // size of seen that triggers the next prune
	local     map[netip.Addr]bool
	localRead time.Time
}

// NewVerifier returns a Verifier accepting signatures by any of keys.
func NewVerifier(keys []ed25519.PublicKey) *Verifier {
	return &Verifier{
		keys:    keys,
		now:     time.Now,
		addrs:   net.InterfaceAddrs,
		seen:    make(map[string]time.Time),
		pruneAt: seenPruneSize,
	}
}

// Verify returns nil when r carries a valid signature that is recent,
// addressed to this agent and not seen before, or the reason it doesn't.
func (v *Verifier) Verify(r *http.Request) error {
	header := r.Header.Get(SignatureHeader)
	if header == "" {
		return ErrUnsigned
	}
	ts, nonce, sig, ok := parseSignature(header)
	if !ok {
		return ErrMalformed
	}
	now := v.now()
	signedAt := time.Unix(ts, 0)
	if skew := now.Sub(signedAt); skew > MaxClockSkew || skew < -MaxClockSkew {
		return ErrStale
	}
	msg := payload(r.Method, r.Host, r.RequestURI, ts, nonce)
	if !v.signedByKnownKey(msg, sig) {
		return ErrBadSignature
	}
	// Checked after the signature: only the server's own requests may make
	// the agent re-read its addresses.
	if !v.isLocal(r.Host) {
		return ErrWrongHost
	}
	if !v.remember(nonce, signedAt.Add(MaxClockSkew), now) {
		return ErrReplayed
	}
	return nil
}

// parseSignature splits a "t=<unix>, n=<nonce>, s=<signature>" header.
func parseSignature(header string) (ts int64, nonce string, sig []byte, ok bool) {
	fields := make(map[string]string, 3)
	for _, part := range strings.Split(header, ",") {
		k, val, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			return 0, "", nil, false
		}
		if _, dup := fields[k]; dup {
			return 0, "", nil, false
		}
		fields[k] = val
	}
	if len(fields) != 3 {
		return 0, "", nil, false
	}
	ts, err := strconv.ParseInt(fields["t"], 10, 64)
	if err != nil {
		return 0, "", nil, false
	}
	nonce = fields["n"]
	if raw, err := b64.DecodeString(nonce); err != nil || len(raw) != nonceSize {
		return 0, "", nil, false
	}
	sig, err = b64.DecodeString(fields["s"])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return 0, "", nil, false
	}
	return ts, nonce, sig, true
}

// signedByKnownKey reports whether sig is a signature of msg by any key.
func (v *Verifier) signedByKnownKey(msg, sig []byte) bool {
	for _, key := range v.keys {
		if ed25519.Verify(key, msg, sig) {
			return true
		}
	}
	return false
}

// isLocal reports whether hostport names one of this agent's own addresses.
// The server dials agents by IP, so a host name never matches.
func (v *Verifier) isLocal(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap().WithZone("")

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.local[addr] {
		return true
	}
	if v.now().Sub(v.localRead) < localAddrRefresh {
		return false
	}
	v.readLocalAddrs()
	return v.local[addr]
}

// readLocalAddrs refreshes the set of this agent's interface addresses. The
// caller holds v.mu.
func (v *Verifier) readLocalAddrs() {
	v.localRead = v.now()
	addrs, err := v.addrs()
	if err != nil {
		return
	}
	local := make(map[netip.Addr]bool, len(addrs))
	for _, a := range addrs {
		var ip net.IP
		switch a := a.(type) {
		case *net.IPNet:
			ip = a.IP
		case *net.IPAddr:
			ip = a.IP
		}
		if addr, ok := netip.AddrFromSlice(ip); ok {
			local[addr.Unmap()] = true
		}
	}
	v.local = local
}

// remember records nonce until expiry and reports whether it was new. A full
// cache refuses new nonces rather than forget ones still acceptable.
func (v *Verifier) remember(nonce string, expiry, now time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, seen := v.seen[nonce]; seen {
		return false
	}
	if len(v.seen) >= v.pruneAt {
		for n, exp := range v.seen {
			if now.After(exp) {
				delete(v.seen, n)
			}
		}
		// Capped, so a full cache is still pruned once its nonces expire.
		v.pruneAt = min(maxSeenNonces, max(seenPruneSize, 2*len(v.seen)))
	}
	if len(v.seen) >= maxSeenNonces {
		return false
	}
	v.seen[nonce] = expiry
	return true
}
