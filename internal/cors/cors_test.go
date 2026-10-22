package cors

import (
	"net/http"
	"testing"
)

// localWildcards is the list a user sets to let a page on any local port in.
var localWildcards = []string{"http://localhost:*", "http://127.0.0.1:*"}

func TestCheckerIsAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		origin  string
		want    bool
	}{
		// "*" is for the CORS header only (see TestCheckerCORSOrigin): it allows no
		// origin here, the local ones included.
		{"wildcard alone allows no other site", []string{"*"}, "http://evil.com", false},
		{"wildcard alone allows no other local port", []string{"*"}, "http://localhost:5173", false},
		{"wildcard next to a named origin", []string{"http://nas.local:8080", "*"}, "http://nas.local:8080", true},
		{"wildcard next to a named origin keeps others out", []string{"http://nas.local:8080", "*"}, "http://evil.com", false},
		{"empty origin always allowed", []string{"http://localhost:8080"}, "", true},
		{"exact match", []string{"http://localhost:8080"}, "http://localhost:8080", true},
		{"exact mismatch", []string{"http://localhost:8080"}, "http://localhost:3000", false},
		{"port wildcard matches", []string{"http://localhost:*"}, "http://localhost:5173", true},
		{"port wildcard different port", []string{"http://localhost:*"}, "http://localhost:8080", true},
		{"port wildcard wrong host", []string{"http://localhost:*"}, "http://example.com:5173", false},
		{"port wildcard no port", []string{"http://localhost:*"}, "http://localhost", false},
		{"port wildcard non-digit port", []string{"http://localhost:*"}, "http://localhost:abc", false},
		{"127.0.0.1 wildcard", []string{"http://127.0.0.1:*"}, "http://127.0.0.1:3000", true},
		{"multiple patterns", []string{"http://localhost:*", "http://127.0.0.1:*"}, "http://127.0.0.1:8080", true},
		{"multiple patterns miss", []string{"http://localhost:*", "http://127.0.0.1:*"}, "http://example.com:8080", false},
		{"empty list allows no other origin", nil, "http://localhost:5173", false},
		{"local wildcards", localWildcards, "http://localhost:5173", true},
		{"local wildcards reject external", localWildcards, "http://evil.com:5173", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(tt.origins)
			if got := c.IsAllowed(tt.origin); got != tt.want {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestCheckerCORSOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		origin  string
		want    string
	}{
		{"wildcard returns star", []string{"*"}, "http://anything.com", "*"},
		{"wildcard next to a named origin still returns star", []string{"http://nas.local:8080", "*"}, "http://evil.com", "*"},
		{"allowed returns origin", []string{"http://localhost:*"}, "http://localhost:5173", "http://localhost:5173"},
		{"disallowed returns empty", []string{"http://localhost:*"}, "http://evil.com:5173", ""},
		{"empty origin returns empty", []string{"http://localhost:*"}, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(tt.origins)
			if got := c.CORSOrigin(tt.origin); got != tt.want {
				t.Errorf("CORSOrigin(%q) = %q, want %q", tt.origin, got, tt.want)
			}
		})
	}
}

func TestCheckerCheckOriginFunc(t *testing.T) {
	c := NewChecker([]string{"http://localhost:*"})
	fn := c.CheckOriginFunc()

	allowed := &http.Request{Header: http.Header{"Origin": []string{"http://localhost:8080"}}}
	if !fn(allowed) {
		t.Error("expected localhost:8080 to be allowed")
	}

	denied := &http.Request{Header: http.Header{"Origin": []string{"http://evil.com:8080"}}}
	if fn(denied) {
		t.Error("expected evil.com:8080 to be denied")
	}

	noOrigin := &http.Request{Header: http.Header{}}
	if !fn(noOrigin) {
		t.Error("expected no Origin header to be allowed")
	}
}

// "*" serves the CORS header and nothing else: a page of another site gets no
// WebSocket and no change through the origin check because of it.
func TestCheckerCheckOriginFuncIgnoresTheWildcard(t *testing.T) {
	fn := NewChecker([]string{"*"}).CheckOriginFunc()

	evil := &http.Request{Host: "localhost:8080", Header: http.Header{"Origin": []string{"http://evil.example"}}}
	if fn(evil) {
		t.Error("* let a page of another site through the origin check")
	}
	devServer := &http.Request{Host: "localhost:8080", Header: http.Header{"Origin": []string{"http://localhost:5173"}}}
	if fn(devServer) {
		t.Error("* let a page of another local port through the origin check")
	}
	own := &http.Request{Host: "localhost:8080", Header: http.Header{"Origin": []string{"http://localhost:8080"}}}
	if !fn(own) {
		t.Error("* dropped the server's own origin")
	}
}

// A page that the server itself served is of the server's own origin: its
// Origin is the address the request was sent to. It needs no entry in the list;
// a page of any other origin, a look-alike included, still does.
func TestCheckerCheckOriginFuncAcceptsTheServersOwnOrigin(t *testing.T) {
	fn := NewChecker(nil).CheckOriginFunc()

	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{"own origin on a LAN address", "192.168.1.50:8080", "http://192.168.1.50:8080", true},
		{"own origin on a name", "nas.local:8080", "http://nas.local:8080", true},
		{"own origin on a name without port", "nas.local", "http://nas.local", true},
		{"own origin on an IPv6 address", "[fd00::1]:8080", "http://[fd00::1]:8080", true},
		{"another port of the same host", "192.168.1.50:8080", "http://192.168.1.50:9999", false},
		{"another host", "192.168.1.50:8080", "http://192.168.1.51:8080", false},
		{"https is not the origin of this server", "192.168.1.50:8080", "https://192.168.1.50:8080", false},
		{"sandboxed page", "192.168.1.50:8080", "null", false},
		{"look-alike with a longer port", "nas.local:8080", "http://nas.local:80801", false},
		{"look-alike with a suffix", "nas.local:8080", "http://nas.local:8080.evil.example", false},
		{"look-alike with a prefix", "nas.local:8080", "http://evil.nas.local:8080", false},
		{"no Origin", "nas.local:8080", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Host: tt.host, Header: http.Header{}}
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if got := fn(r); got != tt.want {
				t.Errorf("Host %q, Origin %q: allowed = %v, want %v", tt.host, tt.origin, got, tt.want)
			}
		})
	}
}

// Behind TLS the page the server served is an https page: only an https origin
// is the server's own then, and an http one is a page of another site.
func TestCheckerUseTLSMakesHTTPSTheOwnOrigin(t *testing.T) {
	c := NewChecker(nil)
	c.UseTLS()
	fn := c.CheckOriginFunc()

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"https origin of this server", "https://192.168.1.50:8080", true},
		{"http origin of this server", "http://192.168.1.50:8080", false},
		{"https look-alike with another port", "https://192.168.1.50:9999", false},
		{"no Origin", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Host: "192.168.1.50:8080", Header: http.Header{}}
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if got := fn(r); got != tt.want {
				t.Errorf("Origin %q: allowed = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

// A request names its target in the Host header. The origin list doubles as the
// list of hosts the server answers to, so a page that reaches the server under
// a name outside the list (DNS rebinding) is turned away.
func TestCheckerIsHostAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		host    string
		want    bool
	}{
		{"local list: localhost with port", localWildcards, "localhost:8080", true},
		{"local list: localhost without port", localWildcards, "localhost", true},
		{"local list: loopback address", localWildcards, "127.0.0.1:22880", true},
		{"local list: case does not matter", localWildcards, "LocalHost:8080", true},
		{"local list: rebinding name", localWildcards, "evil.example:8080", false},
		{"local list: name that merely starts with localhost", localWildcards, "localhost.evil.example:8080", false},
		{"local list: name that merely ends with localhost", localWildcards, "evil-localhost:8080", false},
		{"local list: loopback address as a name prefix", localWildcards, "127.0.0.1.evil.example:8080", false},
		{"local list: userinfo trick", localWildcards, "localhost@evil.example:8080", false},
		{"local list: other address", localWildcards, "192.168.1.50:8080", false},
		// The list alone does not name ::1; a server bound to it adds its own address.
		{"local list alone does not name IPv6 loopback", localWildcards, "[::1]:8080", false},
		{"local list: empty host", localWildcards, "", false},
		{"listed IPv6 loopback", []string{"http://[::1]:*"}, "[::1]:8080", true},
		{"listed IPv6 loopback without port", []string{"http://[::1]:*"}, "[::1]", true},
		{"exact origin lists its host for any port", []string{"http://192.168.1.50:8080"}, "192.168.1.50:9090", true},
		{"replacing the list drops localhost", []string{"http://192.168.1.50:*"}, "localhost:8080", false},
		{"empty list names no host", nil, "localhost:8080", false},
		// "*" is for the CORS header only: it never switches the host check off
		// and names no host itself.
		{"wildcard does not allow every host", []string{"*"}, "evil.example:8080", false},
		{"wildcard alone names no local host", []string{"*"}, "localhost:8080", false},
		{"wildcard next to a named host", []string{"*", "http://nas.local:*"}, "nas.local:8080", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(tt.origins)
			if got := c.IsHostAllowed(tt.host); got != tt.want {
				t.Errorf("IsHostAllowed(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
