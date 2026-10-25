package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted := proxies{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("::1/128")}
	tests := []struct {
		name   string
		remote string
		xff    []string
		want   string
	}{
		{name: "direct client", remote: "192.0.2.1:1234", want: "192.0.2.1"},
		{name: "direct client's X-Forwarded-For is ignored", remote: "192.0.2.1:1234", xff: []string{"203.0.113.9"}, want: "192.0.2.1"},
		{name: "trusted proxy", remote: "10.0.0.1:1234", xff: []string{"192.0.2.1"}, want: "192.0.2.1"},
		{name: "trusted proxy without header", remote: "10.0.0.1:1234", want: "10.0.0.1"},
		{name: "IPv6 loopback proxy", remote: "[::1]:1234", xff: []string{"2001:db8::5"}, want: "2001:db8::5"},
		// The client can prepend anything; only the entries proxies appended
		// (from the right) count, up to the first untrusted address.
		{name: "spoofed entries on the left", remote: "10.0.0.1:1", xff: []string{"203.0.113.9, 192.0.2.1"}, want: "192.0.2.1"},
		{name: "chain of trusted proxies", remote: "10.0.0.1:1", xff: []string{"192.0.2.1, 10.0.0.2", "10.0.0.3"}, want: "192.0.2.1"},
		{name: "every hop trusted", remote: "10.0.0.1:1", xff: []string{"10.0.0.2"}, want: "10.0.0.2"},
		{name: "garbage hop stops the walk", remote: "10.0.0.1:1", xff: []string{"192.0.2.1, nonsense, 10.0.0.2"}, want: "10.0.0.2"},
		{name: "IPv4-mapped IPv6 is unmapped", remote: "10.0.0.1:1", xff: []string{"::ffff:192.0.2.1"}, want: "192.0.2.1"},
		{name: "mapped peer matches an IPv4 proxy", remote: "[::ffff:10.0.0.1]:1", xff: []string{"192.0.2.1"}, want: "192.0.2.1"},
		{name: "zone is dropped", remote: "10.0.0.1:1", xff: []string{"fe80::1%eth0"}, want: "fe80::1"},
		{name: "hop with a port", remote: "10.0.0.1:1", xff: []string{"192.0.2.1:443"}, want: "192.0.2.1"},
		{name: "bracketed IPv6 hop", remote: "10.0.0.1:1", xff: []string{"[2001:db8::1]"}, want: "2001:db8::1"},
		{name: "bracketed IPv6 hop with a port", remote: "10.0.0.1:1", xff: []string{"[2001:db8::1]:443"}, want: "2001:db8::1"},
		{name: "trailing comma", remote: "10.0.0.1:1", xff: []string{"192.0.2.1,"}, want: "192.0.2.1"},
		{name: "spaces around entries", remote: "10.0.0.1:1", xff: []string{" 192.0.2.1 ,10.0.0.2 "}, want: "192.0.2.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := trusted.clientIP(r); got.String() != tt.want {
				t.Errorf("clientIP = %v, want %s", got, tt.want)
			}
		})
	}
}

func TestIsHTTPS(t *testing.T) {
	trusted := proxies{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name   string
		remote string
		proto  string
		want   bool
	}{
		{"plain", "192.0.2.1:1", "", false},
		{"untrusted peer claims https", "192.0.2.1:1", "https", false},
		{"trusted proxy https", "10.0.0.1:1", "https", true},
		{"trusted proxy HTTPS", "10.0.0.1:1", "HTTPS", true},
		{"trusted proxy http", "10.0.0.1:1", "http", false},
		{"first of a list", "10.0.0.1:1", "https, http", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			if tt.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tt.proto)
			}
			if got := trusted.isHTTPS(r); got != tt.want {
				t.Errorf("isHTTPS = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateLimitKey(t *testing.T) {
	for in, want := range map[string]string{
		"192.0.2.1":         "192.0.2.1",
		"2001:db8::1":       "2001:db8::/64",
		"2001:db8::ffff:1":  "2001:db8::/64",
		"2001:db8:0:1::1":   "2001:db8:0:1::/64",
		"fe80::1":           "fe80::/64",
		"198.51.100.255":    "198.51.100.255",
		"2001:db8:1:2:3::4": "2001:db8:1:2::/64",
	} {
		if got := rateLimitKey(netip.MustParseAddr(in)); got != want {
			t.Errorf("rateLimitKey(%s) = %q, want %q", in, got, want)
		}
	}
	if got := rateLimitKey(netip.Addr{}); got != "unknown" {
		t.Errorf("rateLimitKey(invalid) = %q", got)
	}
}
