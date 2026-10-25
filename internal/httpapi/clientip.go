package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// proxies decides what to believe about a request that came through a
// reverse proxy. Forwarding headers are trusted only when the direct peer
// is a configured proxy (WATCHT_TRUSTED_PROXIES); anyone else could set
// them to anything.
type proxies []netip.Prefix

func (p proxies) trusts(addr netip.Addr) bool {
	for _, prefix := range p {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// clientIP returns the address of the client that made the request. When
// the peer is a trusted proxy, it walks X-Forwarded-For from the right
// (the entries the proxies themselves appended) and returns the first
// address that isn't a trusted proxy. Entries further left were written by
// the client and are ignored. An invalid address is returned when nothing
// usable is found (r.RemoteAddr is always set by net/http, so only a
// malformed forwarding chain gets there).
func (p proxies) clientIP(r *http.Request) netip.Addr {
	peer := remoteAddr(r)
	if !peer.IsValid() || !p.trusts(peer) {
		return peer
	}
	ip := peer
	hops := r.Header.Values("X-Forwarded-For")
	for i := len(hops) - 1; i >= 0; i-- {
		parts := strings.Split(hops[i], ",")
		for j := len(parts) - 1; j >= 0; j-- {
			part := strings.TrimSpace(parts[j])
			if part == "" {
				continue // e.g. a trailing comma
			}
			addr, ok := parseHop(part)
			if !ok {
				// A garbled hop: stop at the last address we could believe.
				return ip
			}
			ip = addr
			if !p.trusts(ip) {
				return ip
			}
		}
	}
	return ip
}

// parseHop parses one X-Forwarded-For entry: a bare address, or one with a
// port or IPv6 brackets ("192.0.2.1:443", "[2001:db8::1]"), which some
// proxies send.
func parseHop(s string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		if ap, err := netip.ParseAddrPort(s); err == nil {
			addr = ap.Addr()
		} else if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			if addr, err = netip.ParseAddr(s[1 : len(s)-1]); err != nil {
				return netip.Addr{}, false
			}
		} else {
			return netip.Addr{}, false
		}
	}
	return addr.Unmap().WithZone(""), true
}

// forwardedFromUntrusted reports whether r carries forwarding headers that
// are being ignored because the peer isn't a trusted proxy: a sign that
// WATCHT_TRUSTED_PROXIES is missing.
func (p proxies) forwardedFromUntrusted(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") == "" {
		return false
	}
	peer := remoteAddr(r)
	return peer.IsValid() && !p.trusts(peer)
}

// isHTTPS reports whether the client reached watcht over HTTPS: directly
// over TLS, or through a trusted proxy that says so in X-Forwarded-Proto.
func (p proxies) isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	peer := remoteAddr(r)
	if !peer.IsValid() || !p.trusts(peer) {
		return false
	}
	// With several proxies the header can be a list; the first entry is
	// the scheme the first proxy saw. If a proxy appends instead of
	// overwriting, the client controls that entry, but all it can do is
	// make its own cookie Secure.
	proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap().WithZone("")
}

// rateLimitKey groups clients for the login limiter: one key per IPv4
// address, and one per IPv6 /64, since a single IPv6 host usually controls
// a whole /64 and could otherwise rotate addresses.
func rateLimitKey(ip netip.Addr) string {
	if !ip.IsValid() {
		return "unknown"
	}
	if ip.Is6() {
		prefix, _ := ip.Prefix(64) // can't fail for a valid IPv6 address
		return prefix.String()
	}
	return ip.String()
}
