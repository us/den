package ssrf

import (
	"net"
	"testing"
)

func ip(s string) net.IP {
	p := net.ParseIP(s)
	if p == nil {
		panic("test bug: unparseable IP " + s)
	}
	if v4 := p.To4(); v4 != nil {
		return v4
	}
	return p
}

// TestCanonicalizeHost covers the canonicalization + classification contract:
// lowercasing, bracket/zone/trailing-dot stripping, IPv4-mapped folding, the
// ambiguous-numeric hard reject, and the IDNA refusal.
func TestCanonicalizeHost(t *testing.T) {
	tests := []struct {
		in        string
		wantHost  string
		wantIP    string // "" => DNS hostname (ip==nil)
		wantError bool
	}{
		{in: "LOCALHOST", wantHost: "localhost"},
		{in: "localhost.", wantHost: "localhost"},
		{in: "minio.internal", wantHost: "minio.internal"},
		{in: "127.0.0.1", wantHost: "127.0.0.1", wantIP: "127.0.0.1"},
		{in: "[::1]", wantHost: "::1", wantIP: "::1"},
		{in: "fe80::1%eth0", wantHost: "fe80::1", wantIP: "fe80::1"},
		{in: "::ffff:127.0.0.1", wantHost: "127.0.0.1", wantIP: "127.0.0.1"},
		// Ambiguous numeric shorthands a permissive resolver might deref.
		{in: "127.1", wantError: true},
		{in: "0x7f.1", wantError: true},
		{in: "2130706433", wantError: true},
		{in: "017700000001", wantError: true},
		// IDNA is deliberately unsupported for the trusted endpoint.
		{in: "xn--nxasmq6b.example", wantError: true},
		{in: "exämple.com", wantError: true},
		{in: "", wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			host, gotIP, err := CanonicalizeHost(tc.in)
			if tc.wantError {
				if err == nil {
					t.Fatalf("CanonicalizeHost(%q) = (%q,%v,nil), want error", tc.in, host, gotIP)
				}
				return
			}
			if err != nil {
				t.Fatalf("CanonicalizeHost(%q) unexpected error: %v", tc.in, err)
			}
			if host != tc.wantHost {
				t.Errorf("host = %q, want %q", host, tc.wantHost)
			}
			if tc.wantIP == "" {
				if gotIP != nil {
					t.Errorf("ip = %v, want nil (DNS host)", gotIP)
				}
			} else if !gotIP.Equal(ip(tc.wantIP)) {
				t.Errorf("ip = %v, want %v", gotIP, tc.wantIP)
			}
		})
	}
}

// TestEndpointHost proves the back-compat-safe extraction of full URLs, bare
// host:port, and bare host across the v6-bracket and trailing-dot forms.
func TestEndpointHost(t *testing.T) {
	tests := []struct {
		in       string
		wantHost string
		wantIP   string
		wantErr  bool
	}{
		{in: "http://[::1]:9000", wantHost: "::1", wantIP: "::1"},
		{in: "http://[fd00::1]:9000", wantHost: "fd00::1", wantIP: "fd00::1"},
		{in: "http://localhost.:9000", wantHost: "localhost"},
		{in: "https://s3.amazonaws.com", wantHost: "s3.amazonaws.com"},
		{in: "minio:9000", wantHost: "minio"},
		{in: "127.0.0.1:9000", wantHost: "127.0.0.1", wantIP: "127.0.0.1"},
		{in: "s3.example.com", wantHost: "s3.example.com"},
		{in: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			host, gotIP, err := EndpointHost(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("EndpointHost(%q) = (%q,%v,nil), want error", tc.in, host, gotIP)
				}
				return
			}
			if err != nil {
				t.Fatalf("EndpointHost(%q) unexpected error: %v", tc.in, err)
			}
			if host != tc.wantHost {
				t.Errorf("host = %q, want %q", host, tc.wantHost)
			}
			if tc.wantIP == "" {
				if gotIP != nil {
					t.Errorf("ip = %v, want nil", gotIP)
				}
			} else if !gotIP.Equal(ip(tc.wantIP)) {
				t.Errorf("ip = %v, want %v", gotIP, tc.wantIP)
			}
		})
	}
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", // loopback
		"10.0.0.1", "192.168.1.1", "172.16.0.1", "fd00::1", // private
		"169.254.0.1",   // link-local unicast
		"224.0.0.1",     // IPv4 multicast
		"ff02::1",       // IPv6 multicast / link-local multicast
		"0.0.0.0", "::", // unspecified
		"100.64.0.1",      // CGNAT (RFC6598)
		"198.18.0.1",      // benchmark (RFC2544)
		"169.254.169.254", // AWS/GCP/Azure metadata
		"169.254.169.253", // Azure alternate metadata
		"100.100.100.200", // Alibaba metadata
	}
	for _, s := range blocked {
		if !IsBlockedIP(ip(s)) {
			t.Errorf("IsBlockedIP(%s) = false, want true", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700::1"}
	for _, s := range allowed {
		if IsBlockedIP(ip(s)) {
			t.Errorf("IsBlockedIP(%s) = true, want false (public)", s)
		}
	}
}

// TestExemptionOrdering is the security-critical table: neverExempt ranges can
// NEVER be unlocked even when pinned and allow=true; loopback/private/CGNAT are
// exemptable but only when allow=true AND the IP is a pinned-set member.
func TestExemptionOrdering(t *testing.T) {
	loopback := ip("127.0.0.1")
	lan := ip("10.0.0.1")
	metadata := ip("169.254.169.254")
	linkLocal := ip("fe80::1")
	multicast := ip("224.0.0.1")
	unspecified := ip("0.0.0.0")
	public := ip("8.8.8.8")

	tests := []struct {
		name   string
		ip     net.IP
		allow  bool
		pinned []net.IP
		want   bool
	}{
		{"public always permitted", public, false, nil, true},
		{"loopback blocked by default", loopback, false, nil, false},
		{"loopback exempt when pinned+allow", loopback, true, []net.IP{loopback}, true},
		{"loopback refused when allow but not pinned", loopback, true, []net.IP{lan}, false},
		{"loopback refused when pinned but allow=false", loopback, false, []net.IP{loopback}, false},
		{"lan exempt when pinned+allow", lan, true, []net.IP{lan}, true},
		{"metadata never exempt even if pinned+allow", metadata, true, []net.IP{metadata}, false},
		{"link-local never exempt even if pinned+allow", linkLocal, true, []net.IP{linkLocal}, false},
		{"multicast never exempt even if pinned+allow", multicast, true, []net.IP{multicast}, false},
		{"unspecified never exempt even if pinned+allow", unspecified, true, []net.IP{unspecified}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DialPermitted(tc.ip, tc.allow, tc.pinned); got != tc.want {
				t.Errorf("DialPermitted(%v, allow=%v, pinned=%v) = %v, want %v",
					tc.ip, tc.allow, tc.pinned, got, tc.want)
			}
		})
	}
}

func TestFirstNeverExempt(t *testing.T) {
	if got := FirstNeverExempt([]net.IP{ip("127.0.0.1"), ip("10.0.0.1")}); got != nil {
		t.Errorf("FirstNeverExempt(loopback,lan) = %v, want nil (both exemptable)", got)
	}
	if got := FirstNeverExempt([]net.IP{ip("8.8.8.8")}); got != nil {
		t.Errorf("FirstNeverExempt(public) = %v, want nil", got)
	}
	got := FirstNeverExempt([]net.IP{ip("127.0.0.1"), ip("169.254.169.254"), ip("fe80::1")})
	if got == nil || !got.Equal(ip("169.254.169.254")) {
		t.Errorf("FirstNeverExempt = %v, want 169.254.169.254 (first crown-jewel)", got)
	}
}

func TestResolvePinnedSet(t *testing.T) {
	// IP literal endpoint: returns itself, resolver never called.
	called := false
	set, err := ResolvePinnedSet(func(string) ([]net.IP, error) {
		called = true
		return nil, nil
	}, "http://127.0.0.1:9000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("resolver called for an IP-literal endpoint")
	}
	if len(set) != 1 || !set[0].Equal(ip("127.0.0.1")) {
		t.Fatalf("set = %v, want [127.0.0.1]", set)
	}

	// DNS endpoint: full resolved set is pinned.
	want := []net.IP{ip("10.0.0.5"), ip("10.0.0.6")}
	set, err = ResolvePinnedSet(func(host string) ([]net.IP, error) {
		if host != "minio.lan" {
			t.Fatalf("resolve called with %q, want minio.lan", host)
		}
		return want, nil
	}, "http://minio.lan:9000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set) != 2 || !set[0].Equal(want[0]) || !set[1].Equal(want[1]) {
		t.Fatalf("set = %v, want %v", set, want)
	}

	// Empty resolution is an error (never silently unpinned).
	if _, err := ResolvePinnedSet(func(string) ([]net.IP, error) {
		return nil, nil
	}, "http://minio.lan:9000"); err == nil {
		t.Error("ResolvePinnedSet with empty resolution: want error, got nil")
	}
}
