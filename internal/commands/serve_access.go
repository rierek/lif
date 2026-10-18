package commands

import (
	"cmp"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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
//
// A proxy in front of a loopback listener puts it on the network too, and the
// listener cannot see that: every request comes from loopback. The operator
// says so with --require-token or --public-url, which give such a listener the
// rules of a network one (see remote). Without them a request that carries
// proxy headers is refused.
type serveAccess struct {
	addr                *net.TCPAddr
	loopback            bool
	requireToken        bool // --require-token, or implied by --public-url
	allowRemoteTerminal bool
	allowRemoteAgent    bool
	token               string // empty unless the server is remote
	tokenSource         string // where a token the operator brought comes from; empty for a random one
	tokenDerived        bool   // token is derived from the one in tokenSource (see deriveToken)
	publicBase          string // --public-url without a trailing slash; empty without one
	publicOrigin        string // scheme://host[:port] of publicBase
	scheme              string // "https" when the server speaks TLS itself, else "http"
}

// serveOptions are the access choices of the operator. Every one of them is a
// flag: no key of a committed .lif.yml may open the server.
type serveOptions struct {
	allowRemoteTerminal bool
	allowRemoteAgent    bool
	requireToken        bool
	tokenFile           string   // --token-file
	envToken            string   // LIF_SERVE_TOKEN
	tlsCert             string   // --tls-cert
	tlsKey              string   // --tls-key
	publicURL           *url.URL // nil unless --public-url
}

// parsePublicURL checks the value of --public-url: the address of the server
// as a proxy in front of it shows it, an absolute http or https URL that may
// carry a path (the proxy strips it) but no query or fragment. The host is
// lower case and a default port is dropped, as in the Origin header of a
// browser, and the path loses its trailing slash.
func parsePublicURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, fmt.Errorf("invalid --public-url %q: must be an absolute http:// or https:// URL without query or fragment", s)
	}
	u.Host = strings.ToLower(u.Host)
	if port := u.Port(); (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		u.Host = strings.TrimSuffix(u.Host, ":"+port)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u, nil
}

// newServeAccess decides the access rules from the address of the listener.
func newServeAccess(addr net.Addr, opts serveOptions) (serveAccess, error) {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return serveAccess{}, fmt.Errorf("listener address %v is not a TCP address", addr)
	}
	a := serveAccess{
		addr:                tcp,
		loopback:            tcp.IP.IsLoopback(),
		requireToken:        opts.requireToken || opts.publicURL != nil,
		allowRemoteTerminal: opts.allowRemoteTerminal,
		allowRemoteAgent:    opts.allowRemoteAgent,
		scheme:              "http",
	}
	if opts.tlsCert != "" {
		a.scheme = "https"
	}
	if u := opts.publicURL; u != nil {
		a.publicBase = u.String()
		a.publicOrigin = u.Scheme + "://" + u.Host
	}
	if a.remote() {
		token, source, err := loadToken(opts.tokenFile, opts.envToken)
		if err != nil {
			return serveAccess{}, err
		}
		// US-16: the opt-in for a remote shell gives a new token, so a client
		// that got the plain one before keeps out. The derived token is stable,
		// as the token it comes from is.
		if source != "" && (a.allowRemoteTerminal || a.allowRemoteAgent) {
			token = deriveToken(token, a.allowRemoteTerminal, a.allowRemoteAgent)
			a.tokenDerived = true
		}
		a.token, a.tokenSource = token, source
	}
	return a, nil
}

// serveTokenEnv is the environment variable that holds an access token the
// operator brings (see loadToken).
const serveTokenEnv = "LIF_SERVE_TOKEN"

// minTokenLength is the least a token the operator brings may have, in
// characters.
const minTokenLength = 32

// newRandomToken returns a fresh access token: 32 random bytes, base64url.
func newRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate the access token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// loadToken returns the access token and where it comes from. An operator who
// wants it to survive a restart brings one: tokenFile (--token-file) wins over
// envToken (LIF_SERVE_TOKEN); a missing token file is created with a random
// token. The source is the path or the variable name. Without both the token is
// random and the source is empty.
func loadToken(tokenFile, envToken string) (token, source string, err error) {
	switch {
	case tokenFile != "":
		token, err = readOrCreateTokenFile(tokenFile)
		return token, tokenFile, err
	case envToken != "":
		token = strings.TrimSpace(envToken)
		if len(token) < minTokenLength {
			return "", "", fmt.Errorf("token from %s is shorter than %d characters", serveTokenEnv, minTokenLength)
		}
		return token, serveTokenEnv, nil
	}
	token, err = newRandomToken()
	return token, "", err
}

func readOrCreateTokenFile(path string) (string, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return createTokenFile(path)
	}
	if err != nil {
		return "", fmt.Errorf("failed to read the token file: %w", err)
	}
	// The token opens the server, so a file other users can read is refused.
	// Windows has no such modes.
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm&0o077 != 0 {
		return "", fmt.Errorf("token file %s is accessible by other users (mode %04o); run chmod 600 %s", path, perm, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read the token file: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if len(token) < minTokenLength {
		return "", fmt.Errorf("token from %s is shorter than %d characters", path, minTokenLength)
	}
	return token, nil
}

func createTokenFile(path string) (string, error) {
	token, err := newRandomToken()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("failed to create the token file: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to create the token file: %w", err)
	}
	if _, err := f.WriteString(token + "\n"); err != nil {
		f.Close()
		return "", fmt.Errorf("failed to write the token file %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to write the token file %s: %w", path, err)
	}
	return token, nil
}

// deriveToken is the token of a server started with a remote opt-in on top of
// a token the operator brought: a keyed hash of the opt-ins, with the brought
// token as the key. The plain token then no longer opens the server, and the
// same flags give the same token after a restart.
func deriveToken(base string, terminal, agent bool) string {
	mac := hmac.New(sha256.New, []byte(base))
	fmt.Fprintf(mac, "allow-remote-terminal=%t,allow-remote-agent=%t", terminal, agent)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// remote reports whether the server is reachable by more than the user of this
// machine: its listener is beyond loopback, or the operator announced a proxy
// in front of it (--require-token, --public-url). Remote servers ask for the
// token and serve the terminal only on request.
func (a serveAccess) remote() bool {
	return !a.loopback || a.requireToken
}

// terminalEnabled reports whether the terminal is served at all.
func (a serveAccess) terminalEnabled() bool {
	return !a.remote() || a.allowRemoteTerminal
}

// agentEnabled reports whether agent sessions are served at all. An agent
// session in act mode runs commands as the user running lif-serve, which is a
// shell like the terminal, so a remote server refuses them unless the operator
// asks for them with --allow-remote-agent.
func (a serveAccess) agentEnabled() bool {
	return !a.remote() || a.allowRemoteAgent
}

// secure reports whether the token travels encrypted: the server speaks TLS
// itself, or the public URL of the proxy in front of it is an https one. The
// cookie then carries the Secure flag, and the log drops the clear-text
// warning.
func (a serveAccess) secure() bool {
	return a.scheme == "https" || strings.HasPrefix(a.publicBase, "https://")
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
	own := a.scheme + "://" + net.JoinHostPort(a.addr.IP.String(), "*")
	if slices.Contains(origins, own) {
		return origins
	}
	return append(slices.Clone(origins), own)
}

// withPublicOrigin adds the origin of --public-url to the origin list: a page
// the UI served under that address is of that origin, and the list doubles as
// the list of hosts the server answers to. The list passed in is not modified.
func (a serveAccess) withPublicOrigin(origins []string) []string {
	if a.publicOrigin == "" || slices.Contains(origins, a.publicOrigin) {
		return origins
	}
	return append(slices.Clone(origins), a.publicOrigin)
}

// graphqlEndpoint is the URL of the GraphQL API the playground page talks to:
// absolute behind a public URL, because the page then lives under a prefix
// that a root-relative path would leave.
func (a serveAccess) graphqlEndpoint() string {
	return a.publicBase + "/api/graphql"
}

// middleware returns the guards that run in front of every route. A refused
// request reaches no handler and takes no WebSocket slot.
func (a serveAccess) middleware(checker *cors.Checker) []gin.HandlerFunc {
	var guards []gin.HandlerFunc
	if !a.remote() {
		guards = append(guards, proxyGuard())
	}
	if a.token != "" {
		guards = append(guards, tokenGuard(a.token, a.addr.Port, a.secure()))
	}
	return append(guards, originGuard(checker, a.loopback))
}

// proxyHeaders are the request headers a proxy in front of the server adds:
// the standard forwarding headers and the one of tailscale serve.
var proxyHeaders = []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "Forwarded", "Tailscale-User-Login"}

// proxyGuard refuses a request that came through a proxy while the server asks
// for no token. A proxy connects from loopback, so a loopback server without a
// token would serve every client of the proxy, with its terminal.
func proxyGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, h := range proxyHeaders {
			if c.Request.Header.Get(h) != "" {
				deny(c, http.StatusForbidden, "lif-serve: this request came through a proxy, but lif-serve runs without an access token. Start it with --public-url <url> or --require-token.")
				return
			}
		}
		c.Next()
	}
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
// servers on one host. The cookie is Secure when the token travels encrypted
// (see serveAccess.secure).
func tokenGuard(token string, port int, secure bool) gin.HandlerFunc {
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
			http.SetCookie(c.Writer, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
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
	if a.loopback && a.requireToken {
		add("Access: token required (--require-token or --public-url)")
	}
	if a.token == "" {
		serverURL := a.scheme + "://" + net.JoinHostPort(a.addr.IP.String(), port)
		add("Starting server at %s/", serverURL)
		add("GraphQL Playground: %s/playground", serverURL)
	} else {
		// Every request needs the token, so the first address a person opens
		// carries it. Without it the server answers 401.
		serverURL := cmp.Or(a.publicBase, a.scheme+"://"+net.JoinHostPort(a.networkHost(interfaceAddrs), port))
		warning := "WARNING: anyone who can reach this port and has the access token can read and change items and run agent sessions as the user running lif-serve."
		if !a.secure() {
			warning += " There is no TLS, so the token travels in clear text."
		}
		add("%s Prefer the default loopback address with an SSH tunnel or a VPN.", warning)
		if a.tokenSource != "" && !a.tokenDerived {
			add("Access token: read from %s", a.tokenSource)
			add("Open the UI: %s/", serverURL)
			add("GraphQL Playground: %s/playground", serverURL)
		} else {
			if a.tokenDerived {
				add("Access token: %s (derived from %s for the remote opt-ins)", a.token, a.tokenSource)
			} else {
				add("Access token: %s", a.token)
			}
			add("Open the UI: %s/?token=%s", serverURL, a.token)
			add("GraphQL Playground: %s/playground?token=%s", serverURL, a.token)
		}
	}
	add("Allowed origins: %s", strings.Join(origins, ", "))
	if slices.Contains(origins, "*") {
		add("WARNING: allowed origins include *: every web page can read the answers of this server. Only the UI of this server and the other listed origins can change items or use the terminal, a shell as the user running lif-serve.")
	}
	switch {
	case !a.remote():
		add("Agent: enabled")
	case a.allowRemoteAgent:
		add("Agent: enabled (--allow-remote-agent). WARNING: a client with the access token can run commands through agent sessions as the user running lif-serve.")
	default:
		add("Agent: disabled (the server is reachable from the network). Add --allow-remote-agent to enable it.")
	}
	switch {
	case !a.remote():
		add("Terminal: enabled")
	case a.allowRemoteTerminal:
		add("Terminal: enabled (--allow-remote-terminal). WARNING: a client with the access token gets a shell as the user running lif-serve.")
	default:
		add("Terminal: disabled (the server is reachable from the network). Add --allow-remote-terminal to enable it.")
	}
	return lines
}
