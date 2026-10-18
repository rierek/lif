package commands

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rierek/lif/internal/cors"
)

// serveAccess holds what the address the listener really got decides about
// who may use the server.
//
// A loopback listener is reachable from this machine only. It relies on the
// Host and Origin checks, which turn away web pages: a page of another site,
// and a page under a name that resolves to this machine (DNS rebinding).
//
// Any other listener is reachable from the network. There those checks prove
// nothing, because a script sends whatever headers it likes, so every client
// must present a token generated at startup. The terminal, a shell as the
// server user, is served there only when the operator asks for it.
type serveAccess struct {
	addr                *net.TCPAddr
	loopback            bool
	allowRemoteTerminal bool
	token               string // empty on a loopback listener
}

// newServeAccess decides the access rules from the address of the listener.
func newServeAccess(addr net.Addr, allowRemoteTerminal bool) (serveAccess, error) {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return serveAccess{}, fmt.Errorf("listener address %v is not a TCP address", addr)
	}
	a := serveAccess{addr: tcp, loopback: tcp.IP.IsLoopback(), allowRemoteTerminal: allowRemoteTerminal}
	if !a.loopback {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return serveAccess{}, fmt.Errorf("failed to generate the access token: %w", err)
		}
		a.token = base64.RawURLEncoding.EncodeToString(b)
	}
	return a, nil
}

// terminalEnabled reports whether the terminal is served at all.
func (a serveAccess) terminalEnabled() bool {
	return a.loopback || a.allowRemoteTerminal
}

// withOwnAddress adds the address a loopback listener is bound to to the
// origin list. ::1 and 127.0.0.2 are loopback addresses too, but the default
// list names only localhost and 127.0.0.1: without this the URL the server
// prints for them would be refused by the host check. The list passed in is
// not modified.
func (a serveAccess) withOwnAddress(origins []string) []string {
	if !a.loopback {
		return origins
	}
	own := "http://" + net.JoinHostPort(a.addr.IP.String(), "*")
	if slices.Contains(origins, own) {
		return origins
	}
	return append(slices.Clone(origins), own)
}

// middleware returns the guards that run in front of every route. A refused
// request reaches no handler and takes no WebSocket slot.
func (a serveAccess) middleware(checker *cors.Checker) []gin.HandlerFunc {
	var guards []gin.HandlerFunc
	if !a.loopback {
		guards = append(guards, tokenGuard(a.token, a.addr.Port))
	}
	return append(guards, originGuard(checker, a.loopback))
}

// originGuard turns away web pages that must not use the server.
//
// With checkHost, a request whose Host header names a host outside the origin
// list is refused. That stops DNS rebinding, where a page of an attacker's
// site reaches the server under the attacker's name and so counts as the same
// origin for the browser.
//
// A request that can change state (any method but GET, HEAD and OPTIONS, and
// every WebSocket upgrade, since GraphQL mutations travel over those too) is
// refused when its Origin header is neither in the list nor the server's own
// origin (see cors.Checker.CheckOriginFunc). Browsers send Origin on all of
// them. Requests without Origin, such as curl and scripts, pass here; on the
// network the token is what stops them.
func originGuard(checker *cors.Checker, checkHost bool) gin.HandlerFunc {
	originAllowed := checker.CheckOriginFunc()
	return func(c *gin.Context) {
		r := c.Request
		if checkHost && !checker.IsHostAllowed(r.Host) {
			deny(c, http.StatusForbidden, fmt.Sprintf("lif-serve: host %q is not allowed (see --cors-origin)", r.Host))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && changesState(r) && !originAllowed(r) {
			deny(c, http.StatusForbidden, fmt.Sprintf("lif-serve: origin %q is not allowed (see --cors-origin)", origin))
			return
		}
		c.Next()
	}
}

func changesState(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return isWebSocketUpgrade(r)
	}
	return true
}

// tokenGuard refuses every request that does not carry the token: as
// "Authorization: Bearer <token>" (scripts), or in a cookie. The browser gets
// the cookie by opening the address printed at startup once, which ends in
// ?token=<token>; the guard then redirects to the same address without the
// token. The cookie name carries the port, because cookies do not separate
// servers on one host.
func tokenGuard(token string, port int) gin.HandlerFunc {
	cookieName := "lif_token_" + strconv.Itoa(port)
	valid := func(got string) bool {
		return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
	}
	return func(c *gin.Context) {
		r := c.Request
		if h := r.Header.Get("Authorization"); len(h) > len("Bearer ") && strings.EqualFold(h[:len("Bearer ")], "Bearer ") && valid(h[len("Bearer "):]) {
			c.Next()
			return
		}
		if ck, err := r.Cookie(cookieName); err == nil && valid(ck.Value) {
			c.Next()
			return
		}
		if q := r.URL.Query(); valid(q.Get("token")) {
			http.SetCookie(c.Writer, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
			if r.Method != http.MethodGet || isWebSocketUpgrade(r) {
				c.Next()
				return
			}
			q.Del("token")
			target := "/" + strings.TrimLeft(r.URL.EscapedPath(), "/") // never "//host": that would leave this site
			if len(q) > 0 {
				target += "?" + q.Encode()
			}
			c.Redirect(http.StatusSeeOther, target)
			c.Abort()
			return
		}
		c.Header("WWW-Authenticate", "Bearer")
		deny(c, http.StatusUnauthorized, "lif-serve: access token required. Open the address printed at startup; it ends in ?token=<token>.")
	}
}

func deny(c *gin.Context, status int, message string) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.String(status, message)
	c.Abort()
}

// accessLog is gin's request logger without the query string. The access
// token travels in ?token=, and a log is no place for it.
func accessLog(out io.Writer) gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{Output: out, SkipQueryString: true})
}

// registerTerminalDisabled answers /api/terminal with the reason. Left
// unregistered, the route would fall through to the single-page app.
func registerTerminalDisabled(router *gin.Engine) {
	router.GET("/api/terminal", func(c *gin.Context) {
		deny(c, http.StatusForbidden, "lif-serve: the terminal is disabled because the server is reachable from the network. Start it with --allow-remote-terminal to enable it.")
	})
}

// networkHost is the host under which a person on the network opens the
// server: the address of the listener, or for a listener on all interfaces the
// first LAN address of this machine, or localhost on a machine without one.
func (a serveAccess) networkHost(interfaceAddrs []net.Addr) string {
	if !a.addr.IP.IsUnspecified() {
		return a.addr.IP.String()
	}
	if ip := firstGlobalUnicastIPv4(interfaceAddrs); ip != "" {
		return ip
	}
	return "localhost"
}

// firstGlobalUnicastIPv4 returns the first IPv4 address among addrs that is
// not loopback, link-local or unspecified. A private LAN address counts.
func firstGlobalUnicastIPv4(addrs []net.Addr) string {
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsGlobalUnicast() {
			return n.IP.String()
		}
	}
	return ""
}

// startupLines are the lines the server prints before it starts serving.
// They name the address the server really listens on and what that exposes.
// interfaceAddrs are the addresses of this machine (net.InterfaceAddrs): a
// listener on all interfaces is shown under the first LAN address among them.
func (a serveAccess) startupLines(origins []string, interfaceAddrs []net.Addr) []string {
	port := strconv.Itoa(a.addr.Port)

	var lines []string
	add := func(format string, args ...any) {
		lines = append(lines, "[lif] "+fmt.Sprintf(format, args...))
	}

	switch {
	case a.loopback:
		add("Listening on %s (loopback only: reachable from this machine only)", a.addr)
	case a.addr.IP.IsUnspecified():
		add("Listening on %s (ALL network interfaces: reachable from the network)", a.addr)
	default:
		add("Listening on %s (non-loopback address: reachable from the network)", a.addr)
	}
	if a.loopback {
		serverURL := "http://" + net.JoinHostPort(a.addr.IP.String(), port)
		add("Starting server at %s/", serverURL)
		add("GraphQL Playground: %s/playground", serverURL)
	} else {
		// Every request needs the token, so the first address a person opens
		// carries it. Without it the server answers 401.
		serverURL := "http://" + net.JoinHostPort(a.networkHost(interfaceAddrs), port)
		add("WARNING: anyone who can reach this port and has the access token can read and change items and run agent sessions as the user running lif-serve. There is no TLS, so the token travels in clear text. Prefer the default loopback address with an SSH tunnel or a VPN.")
		add("Access token: %s", a.token)
		add("Open the UI: %s/?token=%s", serverURL, a.token)
		add("GraphQL Playground: %s/playground?token=%s", serverURL, a.token)
	}
	add("Allowed origins: %s", strings.Join(origins, ", "))
	if slices.Contains(origins, "*") {
		add("WARNING: allowed origins include *: every web page can read the answers of this server. Only the UI of this server and the other listed origins can change items or use the terminal, a shell as the user running lif-serve.")
	}
	if slices.ContainsFunc(origins, func(o string) bool { return strings.HasSuffix(o, ":*") }) {
		add("NOTE: a web page from any port of these hosts, such as another local dev server, may change items (which runs project hooks) and use the terminal. List only the origins you need.")
	}
	switch {
	case a.loopback:
		add("Terminal: enabled")
	case a.allowRemoteTerminal:
		add("Terminal: enabled (--allow-remote-terminal). WARNING: a client with the access token gets a shell as the user running lif-serve.")
	default:
		add("Terminal: disabled (the server is reachable from the network). Add --allow-remote-terminal to enable it.")
	}
	return lines
}
