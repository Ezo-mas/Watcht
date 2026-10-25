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
			addr, err := netip.ParseAddr(strings.TrimSpace(parts[j]))
			if err != nil {
				// A garbled hop: stop at the last address we could believe.
				return ip
			}
			ip = addr.Unmap().WithZone("")
			if !p.trusts(ip) {
				return ip
			}
		}
	}
	return ip
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
	// what the client used.
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
