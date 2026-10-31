package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func mustProxies(t *testing.T, s string) TrustedProxies {
	t.Helper()
	p, err := ParseTrustedProxies(s)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", s, err)
	}
	return p
}

func TestParseTrustedProxies(t *testing.T) {
	p := mustProxies(t, " 10.0.0.0/8, 172.18.0.5 ,, fd00::/64, ::ffff:192.0.2.1")
	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"10.1.2.3", true},
		{"172.18.0.5", true},
		{"172.18.0.6", false},
		{"fd00::1", true},
		{"fd00:0:0:1::1", false},
		{"192.0.2.1", true}, // a mapped address is matched as IPv4
	} {
		if got := p.trusts(netip.MustParseAddr(tc.addr)); got != tc.want {
			t.Errorf("trusts(%s) = %v, want %v", tc.addr, got, tc.want)
		}
	}

	for _, bad := range []string{"10.0.0.0/33", "not-an-ip", "10.0.0.1:80"} {
		if _, err := ParseTrustedProxies(bad); err == nil {
			t.Errorf("ParseTrustedProxies(%q) succeeded, want an error", bad)
		}
	}
	if p, err := ParseTrustedProxies(""); err != nil || len(p) != 0 {
		t.Errorf("empty list = %v, %v; want none", p, err)
	}
}

func TestClientAddr(t *testing.T) {
	proxies := mustProxies(t, "10.0.0.0/8")
	for _, tc := range []struct {
		name    string
		proxies TrustedProxies
		remote  string
		xff     []string
		want    string
	}{
		{"no proxies: forwarding header ignored", nil, "203.0.113.7:4000", []string{"198.51.100.1"}, "203.0.113.7"},
		{"untrusted peer: header ignored", proxies, "203.0.113.7:4000", []string{"198.51.100.1"}, "203.0.113.7"},
		{"trusted peer: client from header", proxies, "10.0.0.2:4000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"spoofed entries left of the real client are skipped", proxies, "10.0.0.2:4000", []string{"1.1.1.1, 198.51.100.1"}, "198.51.100.1"},
		{"chain of trusted proxies", proxies, "10.0.0.2:4000", []string{"198.51.100.1, 10.0.0.9", "10.0.0.3"}, "198.51.100.1"},
		{"entry with port", proxies, "10.0.0.2:4000", []string{"[2001:db8::7]:443"}, "2001:db8::7"},
		{"bracketed entry without port", proxies, "10.0.0.2:4000", []string{"[2001:db8::7]"}, "2001:db8::7"},
		{"bracketed IPv4 entry: last trusted hop", proxies, "10.0.0.2:4000", []string{"[198.51.100.1]"}, "10.0.0.2"},
		{"malformed entry: last trusted hop", proxies, "10.0.0.2:4000", []string{"garbage, 10.0.0.9"}, "10.0.0.9"},
		{"trusted peer without header", proxies, "10.0.0.2:4000", nil, "10.0.0.2"},
		{"mapped IPv4 peer", proxies, "[::ffff:10.0.0.2]:4000", []string{"198.51.100.1"}, "198.51.100.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
			r.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			got, ok := tc.proxies.ClientAddr(r)
			if !ok || got.String() != tc.want {
				t.Errorf("ClientAddr = %v, %v; want %s", got, ok, tc.want)
			}
		})
	}

	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "@"
	if _, ok := proxies.ClientAddr(r); ok {
		t.Error("an unparsable peer address should not be ok")
	}
}

func TestIsHTTPS(t *testing.T) {
	proxies := mustProxies(t, "10.0.0.2")
	for _, tc := range []struct {
		name   string
		remote string
		proto  string
		tls    bool
		want   bool
	}{
		{"direct TLS", "203.0.113.7:4000", "", true, true},
		{"plain HTTP", "203.0.113.7:4000", "", false, false},
		{"untrusted peer claiming https", "203.0.113.7:4000", "https", false, false},
		{"trusted proxy terminating TLS", "10.0.0.2:4000", "https", false, true},
		{"trusted proxy over http", "10.0.0.2:4000", "http", false, false},
		{"nearest proxy's value wins", "10.0.0.2:4000", "https, http", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
			r.RemoteAddr = tc.remote
			if tc.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := proxies.IsHTTPS(r); got != tc.want {
				t.Errorf("IsHTTPS = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLimiterKeyBucketsIPv6ByPrefix(t *testing.T) {
	a := clientBucket(netip.MustParseAddr("2001:db8:1:2::10"))
	b := clientBucket(netip.MustParseAddr("2001:db8:1:2:ffff::1"))
	c := clientBucket(netip.MustParseAddr("2001:db8:1:3::10"))
	if a != b {
		t.Errorf("addresses in one /64 got different keys: %s, %s", a, b)
	}
	if a == c {
		t.Errorf("addresses in different /64s share key %s", a)
	}
	if k := clientBucket(netip.MustParseAddr("192.0.2.1")); k != "192.0.2.1" {
		t.Errorf("IPv4 key = %s, want the address", k)
	}
}

// Behind NAT64 every IPv4 client shows up inside 64:ff9b::/96, so each is
// bucketed by the IPv4 address it carries, as if it had connected directly.
func TestLimiterKeyUnwrapsNAT64(t *testing.T) {
	for _, tc := range []struct{ addr, want string }{
		{"64:ff9b::192.0.2.1", "192.0.2.1"},
		{"64:ff9b::192.0.2.2", "192.0.2.2"},
		{"64:ff9b::1:c000:201", "64:ff9b::/64"}, // outside the /96: an ordinary IPv6 host
	} {
		if got := clientBucket(netip.MustParseAddr(tc.addr)); got != tc.want {
			t.Errorf("clientBucket(%s) = %s, want %s", tc.addr, got, tc.want)
		}
	}
}
