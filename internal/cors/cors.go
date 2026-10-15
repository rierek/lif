package cors

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// DefaultOrigins are the default allowed origins for CORS and WebSocket.
// They cover common local development scenarios.
var DefaultOrigins = []string{
	"http://localhost:*",
	"http://127.0.0.1:*",
}

// Checker validates HTTP origins against an allowlist.
// Patterns support a trailing `:*` wildcard to match any port.
// The hosts the patterns name also tell which Host headers the server
// answers to (see IsHostAllowed).
//
// The entry "*" serves the CORS response header only: CORSOrigin answers "*"
// for every origin, so any page may read what the server answers. Nothing else
// follows from it. IsAllowed, IsHostAllowed and CheckOriginFunc look at the
// named entries, and "*" adds the default local origins to them, because
// "every origin" includes those.
type Checker struct {
	allowAll bool
	patterns []pattern
	hosts    []string // host names of the patterns, lower case, no port
}

type pattern struct {
	prefix   string // scheme + host portion before port wildcard
	exact    string // non-empty for exact matches
	wildcard bool   // true if port is wildcarded
}

// NewChecker creates a Checker from a list of origin patterns.
// Supported patterns:
//   - "*" sends the CORS header for all origins and allows nothing else
//   - "http://localhost:5173" exact match
//   - "http://localhost:*" matches any port on localhost
func NewChecker(origins []string) *Checker {
	c := &Checker{}
	for _, o := range origins {
		if o == "*" {
			c.allowAll = true
			continue
		}
		c.add(o)
	}
	if c.allowAll {
		for _, o := range DefaultOrigins {
			c.add(o)
		}
	}
	return c
}

func (c *Checker) add(origin string) {
	c.patterns = append(c.patterns, parsePattern(origin))
	if h := patternHost(origin); h != "" {
		c.hosts = append(c.hosts, h)
	}
}

func parsePattern(s string) pattern {
	if strings.HasSuffix(s, ":*") {
		return pattern{
			prefix:   s[:len(s)-1], // keep "scheme://host:" prefix
			wildcard: true,
		}
	}
	return pattern{exact: s}
}

// patternHost returns the lower-case host name of an origin pattern, without
// scheme and port, or "" if the pattern names none.
func patternHost(s string) string {
	u, err := url.Parse(strings.TrimSuffix(s, ":*"))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// IsAllowed returns true if the given origin is named in the list. A request
// without an origin passes. The entry "*" does not allow an origin here.
func (c *Checker) IsAllowed(origin string) bool {
	if origin == "" {
		// Same-origin requests (no Origin header) are always allowed
		return true
	}
	for _, p := range c.patterns {
		if p.wildcard {
			if strings.HasPrefix(origin, p.prefix) {
				port := origin[len(p.prefix):]
				if port != "" && isDigits(port) {
					return true
				}
			}
		} else if p.exact == origin {
			return true
		}
	}
	return false
}

// IsHostAllowed returns true if the Host header of a request names a host of
// the list, whatever the port. A page that reaches the server under any other
// name, as in DNS rebinding where an attacker's name resolves to this machine,
// is not allowed. An absent Host is not allowed, and "*" does not change that.
func (c *Checker) IsHostAllowed(hostHeader string) bool {
	host, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		host = strings.Trim(hostHeader, "[]") // no port: a bare name or "[::1]"
	}
	host = strings.ToLower(host)
	for _, h := range c.hosts {
		if h == host {
			return true
		}
	}
	return false
}

// CheckOriginFunc returns a function suitable for gorilla/websocket's Upgrader.CheckOrigin.
// It accepts a request without an Origin, an Origin named in the list, and the
// server's own origin: the address the request was sent to. A page that the
// server itself served is of that origin, so the UI built into the server
// needs no entry. Only http counts, as the server has no TLS.
func (c *Checker) CheckOriginFunc() func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return c.IsAllowed(origin) || origin == "http://"+r.Host
	}
}

// CORSOrigin returns the value for the Access-Control-Allow-Origin header.
// If the list has "*", returns "*". If the specific origin is allowed, returns
// it (enabling per-origin CORS). Returns empty string if not allowed.
func (c *Checker) CORSOrigin(origin string) string {
	if c.allowAll {
		return "*"
	}
	if c.IsAllowed(origin) && origin != "" {
		return origin
	}
	return ""
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
