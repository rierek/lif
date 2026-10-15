package cors

import (
	"net/http"
	"testing"
)

func TestCheckerIsAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		origin  string
		want    bool
	}{
		{"wildcard allows all", []string{"*"}, "http://evil.com", true},
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
		{"default origins", DefaultOrigins, "http://localhost:5173", true},
		{"default origins reject external", DefaultOrigins, "http://evil.com:5173", false},
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
		{"default list: localhost with port", DefaultOrigins, "localhost:8080", true},
		{"default list: localhost without port", DefaultOrigins, "localhost", true},
		{"default list: loopback address", DefaultOrigins, "127.0.0.1:22880", true},
		{"default list: case does not matter", DefaultOrigins, "LocalHost:8080", true},
		{"default list: rebinding name", DefaultOrigins, "evil.example:8080", false},
		{"default list: name that merely starts with localhost", DefaultOrigins, "localhost.evil.example:8080", false},
		{"default list: name that merely ends with localhost", DefaultOrigins, "evil-localhost:8080", false},
		{"default list: loopback address as a name prefix", DefaultOrigins, "127.0.0.1.evil.example:8080", false},
		{"default list: userinfo trick", DefaultOrigins, "localhost@evil.example:8080", false},
		{"default list: other address", DefaultOrigins, "192.168.1.50:8080", false},
		{"default list: IPv6 loopback is not listed", DefaultOrigins, "[::1]:8080", false},
		{"default list: empty host", DefaultOrigins, "", false},
		{"listed IPv6 loopback", []string{"http://[::1]:*"}, "[::1]:8080", true},
		{"listed IPv6 loopback without port", []string{"http://[::1]:*"}, "[::1]", true},
		{"exact origin lists its host for any port", []string{"http://192.168.1.50:8080"}, "192.168.1.50:9090", true},
		{"replacing the list drops localhost", []string{"http://192.168.1.50:*"}, "localhost:8080", false},
		{"wildcard origin allows every host", []string{"*"}, "evil.example:8080", true},
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
