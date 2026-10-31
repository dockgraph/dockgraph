package agentauth

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mustKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	text, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	key, err := ParsePrivateKey(text)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustPublic(t *testing.T, key ed25519.PrivateKey) []ed25519.PublicKey {
	t.Helper()
	keys, err := ParsePublicKeys(PublicKeyText(key))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// localhostOnly makes a verifier see 127.0.0.1 as its only address.
func localhostOnly(v *Verifier) *Verifier {
	v.addrs = func() ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}}, nil
	}
	return v
}

// signed returns a request as an agent receives it, signed by signer for
// the URL given.
func signed(t *testing.T, signer Authorizer, method, url string) *http.Request {
	t.Helper()
	out, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.Authorize(out); err != nil {
		t.Fatal(err)
	}
	return received(out)
}

// received turns an outgoing request into the request the agent's server
// sees: the same method, Host header, request target and headers.
func received(out *http.Request) *http.Request {
	in := httptest.NewRequest(out.Method, out.URL.RequestURI(), nil)
	in.Host = requestHost(out)
	in.Header = out.Header.Clone()
	return in
}

func TestKeyTextRoundTrip(t *testing.T) {
	text, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, privateKeyPrefix) {
		t.Fatalf("private key %q lacks its prefix", text)
	}
	key, err := ParsePrivateKey("\n  " + text + "\n")
	if err != nil {
		t.Fatalf("ParsePrivateKey: %v", err)
	}
	pub := PublicKeyText(key)
	if !strings.HasPrefix(pub, publicKeyPrefix) {
		t.Fatalf("public key %q lacks its prefix", pub)
	}
	keys, err := ParsePublicKeys(pub + "\n")
	if err != nil || len(keys) != 1 || !keys[0].Equal(key.Public()) {
		t.Fatalf("ParsePublicKeys = %v, %v", keys, err)
	}
}

func TestParseKeysRejectsMixUps(t *testing.T) {
	text, _ := GenerateKey()
	key, _ := ParsePrivateKey(text)
	pub := PublicKeyText(key)

	if _, err := ParsePrivateKey(pub); err == nil || !strings.Contains(err.Error(), "public key") {
		t.Errorf("a public key given as the private key: err = %v", err)
	}
	if _, err := ParsePublicKeys(text); err == nil || !strings.Contains(err.Error(), "private key") {
		t.Errorf("the private key given to an agent: err = %v", err)
	}
	if _, err := ParsePublicKeys(pub + "," + text); err == nil {
		t.Error("a private key among public keys must be refused")
	}
	for _, bad := range []string{"", "   ", "hunter2", privateKeyPrefix + "abc", privateKeyPrefix + "!!!"} {
		if _, err := ParsePrivateKey(bad); err == nil {
			t.Errorf("ParsePrivateKey(%q) succeeded", bad)
		}
	}
	for _, bad := range []string{"", " \n ", "hunter2", publicKeyPrefix + "abc"} {
		if _, err := ParsePublicKeys(bad); err == nil {
			t.Errorf("ParsePublicKeys(%q) succeeded", bad)
		}
	}
	// Errors never echo key material.
	if _, err := ParsePrivateKey("secret-value-123"); err != nil && strings.Contains(err.Error(), "secret-value-123") {
		t.Errorf("error echoes its input: %v", err)
	}
}

func TestParsePublicKeysSeveral(t *testing.T) {
	a, b := mustKey(t), mustKey(t)
	keys, err := ParsePublicKeys(PublicKeyText(a) + ",\n " + PublicKeyText(b))
	if err != nil || len(keys) != 2 {
		t.Fatalf("got %d keys, %v", len(keys), err)
	}
}

// A request signed by the server's Signer passes the agent's Verifier over a
// real HTTP round trip, query string and all.
func TestSignedRequestVerifiesOverHTTP(t *testing.T) {
	key := mustKey(t)
	v := NewVerifier(mustPublic(t, key))
	got := make(chan error, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got <- v.Verify(r)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/agent/v1/containers/shop_web.1.t1/logs/history?limit=5&before=2026-01-01T00%3A00%3A00Z", nil)
	if err := NewSigner(key).Authorize(req); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if err := <-got; err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	key := mustKey(t)
	signer := NewSigner(key)
	const url = "http://127.0.0.1:7801/agent/v1/containers/web/logs?tail=10"

	for name, tamper := range map[string]func(r *http.Request){
		"method": func(r *http.Request) { r.Method = http.MethodPost },
		"path":   func(r *http.Request) { r.RequestURI = "/agent/v1/containers/db/logs?tail=10" },
		"query":  func(r *http.Request) { r.RequestURI = "/agent/v1/containers/web/logs?tail=1000" },
		"host":   func(r *http.Request) { r.Host = "127.0.0.2:7801" },
		"timestamp": func(r *http.Request) {
			h := r.Header.Get(SignatureHeader)
			r.Header.Set(SignatureHeader, strings.Replace(h, "t=", "t=1", 1))
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := signed(t, signer, http.MethodGet, url)
			tamper(r)
			err := localhostOnly(NewVerifier(mustPublic(t, key))).Verify(r)
			if !errors.Is(err, ErrBadSignature) && !errors.Is(err, ErrStale) {
				t.Errorf("err = %v, want a rejected signature", err)
			}
		})
	}
}

func TestVerifyRejectsOtherKey(t *testing.T) {
	r := signed(t, NewSigner(mustKey(t)), http.MethodGet, "http://127.0.0.1:7801/agent/v1/info")
	v := localhostOnly(NewVerifier(mustPublic(t, mustKey(t))))
	if err := v.Verify(r); !errors.Is(err, ErrBadSignature) {
		t.Errorf("err = %v, want ErrBadSignature", err)
	}
}

// During a key rotation agents list the old and the new public key.
func TestVerifyAcceptsAnyListedKey(t *testing.T) {
	oldKey, newKey := mustKey(t), mustKey(t)
	keys := append(mustPublic(t, oldKey), mustPublic(t, newKey)...)
	v := localhostOnly(NewVerifier(keys))
	for _, key := range []ed25519.PrivateKey{oldKey, newKey} {
		if err := v.Verify(signed(t, NewSigner(key), http.MethodGet, "http://127.0.0.1:7801/agent/v1/info")); err != nil {
			t.Errorf("Verify: %v", err)
		}
	}
}

func TestVerifyClockSkew(t *testing.T) {
	key := mustKey(t)
	now := time.Now()
	for _, tc := range []struct {
		offset time.Duration
		ok     bool
	}{
		{0, true},
		{-MaxClockSkew + time.Second, true},
		{MaxClockSkew - time.Second, true},
		{-MaxClockSkew - time.Second, false},
		{MaxClockSkew + time.Second, false},
	} {
		signer := NewSigner(key)
		signer.now = func() time.Time { return now.Add(tc.offset) }
		v := localhostOnly(NewVerifier(mustPublic(t, key)))
		v.now = func() time.Time { return now }
		err := v.Verify(signed(t, signer, http.MethodGet, "http://127.0.0.1:7801/agent/v1/info"))
		if tc.ok && err != nil {
			t.Errorf("offset %s: %v", tc.offset, err)
		}
		if !tc.ok && !errors.Is(err, ErrStale) {
			t.Errorf("offset %s: err = %v, want ErrStale", tc.offset, err)
		}
	}
}

func TestVerifyRejectsReplay(t *testing.T) {
	key := mustKey(t)
	v := localhostOnly(NewVerifier(mustPublic(t, key)))
	r := signed(t, NewSigner(key), http.MethodGet, "http://127.0.0.1:7801/agent/v1/stats")
	if err := v.Verify(r); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := v.Verify(r); !errors.Is(err, ErrReplayed) {
		t.Errorf("replay: err = %v, want ErrReplayed", err)
	}
}

// A request signed for one agent is refused by another, even with the Host
// header it was signed for.
func TestVerifyRejectsRequestForAnotherAgent(t *testing.T) {
	key := mustKey(t)
	v := localhostOnly(NewVerifier(mustPublic(t, key)))
	for _, url := range []string{
		"http://10.0.1.7:7801/agent/v1/info",
		"http://[fd00::7]:7801/agent/v1/info",
		"http://tasks.agent:7801/agent/v1/info",
	} {
		if err := v.Verify(signed(t, NewSigner(key), http.MethodGet, url)); !errors.Is(err, ErrWrongHost) {
			t.Errorf("%s: err = %v, want ErrWrongHost", url, err)
		}
	}
	if err := v.Verify(signed(t, NewSigner(key), http.MethodGet, "http://[::ffff:127.0.0.1]:7801/agent/v1/info")); err != nil {
		t.Errorf("mapped local address: %v", err)
	}
}

func TestVerifyRejectsUnsignedAndMalformed(t *testing.T) {
	key := mustKey(t)
	v := localhostOnly(NewVerifier(mustPublic(t, key)))
	good := signed(t, NewSigner(key), http.MethodGet, "http://127.0.0.1:7801/agent/v1/info").Header.Get(SignatureHeader)

	r := httptest.NewRequest(http.MethodGet, "/agent/v1/info", nil)
	r.Host = "127.0.0.1:7801"
	r.Header.Set("Authorization", "Bearer some-token")
	if err := v.Verify(r); !errors.Is(err, ErrUnsigned) {
		t.Errorf("bearer only: err = %v, want ErrUnsigned", err)
	}

	for _, header := range []string{
		"garbage",
		"t=1, n=abc",
		good + ", t=1",
		strings.Replace(good, "t=", "t=x", 1),
		strings.Replace(good, "n=", "n=!", 1),
		strings.Replace(good, "s=", "s=AAAA", 1),
	} {
		r.Header.Set(SignatureHeader, header)
		if err := v.Verify(r); !errors.Is(err, ErrMalformed) {
			t.Errorf("header %q: err = %v, want ErrMalformed", header, err)
		}
	}
}

func TestChainSendsSignatureAndToken(t *testing.T) {
	key := mustKey(t)
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:7801/agent/v1/info", nil)
	if err := (Chain{NewSigner(key), Token("legacy")}).Authorize(req); err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer legacy" {
		t.Errorf("Authorization = %q", got)
	}
	if err := localhostOnly(NewVerifier(mustPublic(t, key))).Verify(received(req)); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// An unknown address re-reads the interfaces at most once per
// localAddrRefresh.
func TestVerifyRefreshesLocalAddressesSparingly(t *testing.T) {
	key := mustKey(t)
	now := time.Now()
	v := NewVerifier(mustPublic(t, key))
	v.now = func() time.Time { return now }
	reads := 0
	v.addrs = func() ([]net.Addr, error) {
		reads++
		return []net.Addr{&net.IPAddr{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	signer := NewSigner(key)
	signer.now = v.now
	for range 3 {
		_ = v.Verify(signed(t, signer, http.MethodGet, "http://10.0.1.7:7801/agent/v1/info"))
	}
	if reads != 1 {
		t.Errorf("interfaces read %d times within %s, want 1", reads, localAddrRefresh)
	}
	now = now.Add(localAddrRefresh + time.Millisecond)
	_ = v.Verify(signed(t, signer, http.MethodGet, "http://10.0.1.7:7801/agent/v1/info"))
	if reads != 2 {
		t.Errorf("interfaces read %d times after %s, want 2", reads, localAddrRefresh)
	}
}

func TestRememberBoundsTheCache(t *testing.T) {
	v := NewVerifier(nil)
	now := time.Now()
	live := now.Add(time.Minute)
	for i := range maxSeenNonces {
		if !v.remember(fmt.Sprint(i), live, now) {
			t.Fatalf("nonce %d refused below the cap", i)
		}
	}
	if v.remember("one-more", live, now) {
		t.Error("a full cache of live nonces must refuse new ones")
	}
	// Once they expire, the cache makes room again.
	later := live.Add(time.Second)
	if !v.remember("after-expiry", later.Add(time.Minute), later) {
		t.Error("expired nonces must be dropped to make room")
	}
	if len(v.seen) != 1 {
		t.Errorf("%d nonces kept, want only the new one", len(v.seen))
	}
}
