package auth

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedProxies lists the reverse proxies (DG_TRUSTED_PROXIES) whose
// X-Forwarded-For and X-Forwarded-Proto headers are believed. A request from
// any other peer is taken at face value: its forwarding headers could have
// been written by anyone, so they are ignored. The zero value trusts no one.
type TrustedProxies []netip.Prefix

// ParseTrustedProxies parses a comma-separated list of IP addresses and CIDR
// prefixes, such as "10.0.0.0/8, 172.18.0.5".
func ParseTrustedProxies(s string) (TrustedProxies, error) {
	var out TrustedProxies
	for _, field := range strings.Split(s, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if strings.Contains(field, "/") {
			p, err := netip.ParsePrefix(field)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy %q: %w", field, err)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(field)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: %w", field, err)
		}
		a = a.Unmap().WithZone("")
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

// trusts reports whether a is one of the trusted proxies.
func (t TrustedProxies) trusts(a netip.Addr) bool {
	a = a.Unmap().WithZone("")
	for _, p := range t {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// ClientAddr returns the address of the client behind r. It is the
// connection's peer, unless the peer is a trusted proxy: then it is the
// rightmost X-Forwarded-For entry that is not itself a trusted proxy.
// Entries further left were written by the client and prove nothing. ok is
// false when the peer address cannot be parsed.
func (t TrustedProxies) ClientAddr(r *http.Request) (addr netip.Addr, ok bool) {
	addr, ok = peerAddr(r)
	if !ok || !t.trusts(addr) {
		return addr, ok
	}
	hops := splitHeaderList(r.Header.Values("X-Forwarded-For"))
	for i := len(hops) - 1; i >= 0; i-- {
		hop, valid := parseHop(hops[i])
		if !valid {
			// The chain is broken here: the last trusted hop is all we know.
			return addr, true
		}
		if !t.trusts(hop) {
			return hop, true
		}
		addr = hop
	}
	return addr, true
}

// IsHTTPS reports whether the client reached the server over HTTPS: directly
// through TLS, or through a trusted proxy that says so in X-Forwarded-Proto.
func (t TrustedProxies) IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	addr, ok := peerAddr(r)
	if !ok || !t.trusts(addr) {
		return false
	}
	// With several proxies the value nearest to this server comes last.
	protos := splitHeaderList(r.Header.Values("X-Forwarded-Proto"))
	return len(protos) > 0 && strings.EqualFold(protos[len(protos)-1], "https")
}

// peerAddr returns the address of the connection's peer.
func peerAddr(r *http.Request) (netip.Addr, bool) {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	return ap.Addr().Unmap().WithZone(""), true
}

// splitHeaderList flattens repeated, comma-separated header values into one
// list, oldest hop first.
func splitHeaderList(values []string) []string {
	var hops []string
	for _, v := range values {
		for _, h := range strings.Split(v, ",") {
			if h = strings.TrimSpace(h); h != "" {
				hops = append(hops, h)
			}
		}
	}
	return hops
}

// parseHop parses one X-Forwarded-For entry: an address, optionally with a
// port ("203.0.113.7:51234", "[2001:db8::1]:443") or an IPv6 address in
// brackets without one ("[2001:db8::1]"), as some proxies write it.
func parseHop(s string) (netip.Addr, bool) {
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap().WithZone(""), true
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap().WithZone(""), true
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		// Brackets hold IPv6 only, as netip.ParseAddrPort requires too.
		if a, err := netip.ParseAddr(s[1 : len(s)-1]); err == nil && a.Is6() {
			return a.Unmap().WithZone(""), true
		}
	}
	return netip.Addr{}, false
}

// ClientKey returns the bucket that per-client limits count r against: the
// client address (see ClientAddr) by clientBucket, or the raw peer address
// when it cannot be parsed.
func (t TrustedProxies) ClientKey(r *http.Request) string {
	if addr, ok := t.ClientAddr(r); ok {
		return clientBucket(addr)
	}
	return r.RemoteAddr
}

// nat64Prefix is the well-known NAT64 prefix (RFC 6052): a translator in
// front of an IPv6-only server shows each IPv4 client as this prefix
// followed by the client's IPv4 address.
var nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")

// clientBucket is the bucket per-client limits count a client address in.
// An IPv6 host is usually given a whole /64, so it is bucketed by that
// prefix; otherwise every address of the prefix would get its own budget. A
// NAT64 address is an IPv4 client, bucketed by that IPv4 address: as a /64,
// every IPv4 client of the translator would share one budget.
func clientBucket(a netip.Addr) string {
	if nat64Prefix.Contains(a) {
		b := a.As16()
		return netip.AddrFrom4([4]byte(b[12:])).String()
	}
	if a.Is6() {
		return netip.PrefixFrom(a, 64).Masked().String()
	}
	return a.String()
}
