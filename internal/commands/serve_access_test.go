package commands

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"

	"github.com/rierek/lif/internal/cors"
	"github.com/rierek/lif/internal/terminal"
	"github.com/rierek/lif/pkg/config"
)

func tcpAddr(ip string, port int) *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
}

func mustAccess(t *testing.T, ip string, allowRemoteTerminal bool) serveAccess {
	t.Helper()
	return mustAccessOpts(t, ip, serveOptions{allowRemoteTerminal: allowRemoteTerminal})
}

func mustAccessOpts(t *testing.T, ip string, opts serveOptions) serveAccess {
	t.Helper()
	a, err := newServeAccess(tcpAddr(ip, 8080), opts)
	if err != nil {
		t.Fatalf("newServeAccess(%s): %v", ip, err)
	}
	return a
}

func mustPublicURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := parsePublicURL(s)
	if err != nil {
		t.Fatalf("parsePublicURL(%q): %v", s, err)
	}
	return u
}

// guardedRequest sends one request through the router. host and origin are the
// Host and Origin headers; an empty origin sends none, like curl.
func guardedRequest(router http.Handler, method, host, origin, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://"+host+path, strings.NewReader("{}"))
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// The address the listener really got decides the access rules, not the
// text of --host: a loopback listener needs no token and serves the terminal,
// every other one needs a token and serves the terminal only on request.
func TestNewServeAccessDecidesFromTheRealAddress(t *testing.T) {
	tests := []struct {
		name         string
		ip           string
		optIn        bool
		wantLoopback bool
		wantTerminal bool
	}{
		{"IPv4 loopback", "127.0.0.1", false, true, true},
		{"IPv6 loopback", "::1", false, true, true},
		{"another loopback address", "127.0.0.2", false, true, true},
		{"loopback, opt-in changes nothing", "127.0.0.1", true, true, true},
		{"all IPv4 interfaces", "0.0.0.0", false, false, false},
		{"all interfaces, dual stack", "::", false, false, false},
		{"LAN address", "192.168.1.50", false, false, false},
		{"all interfaces, terminal opted in", "0.0.0.0", true, false, true},
		{"LAN address, terminal opted in", "192.168.1.50", true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mustAccess(t, tt.ip, tt.optIn)
			if a.loopback != tt.wantLoopback {
				t.Errorf("loopback = %v, want %v", a.loopback, tt.wantLoopback)
			}
			if got := a.terminalEnabled(); got != tt.wantTerminal {
				t.Errorf("terminalEnabled() = %v, want %v", got, tt.wantTerminal)
			}
			if a.loopback && a.token != "" {
				t.Errorf("a loopback listener got the token %q", a.token)
			}
			if !a.loopback && len(a.token) < 32 {
				t.Errorf("a listener on %s has no usable token: %q", tt.ip, a.token)
			}
		})
	}

	first, second := mustAccess(t, "0.0.0.0", false), mustAccess(t, "0.0.0.0", false)
	if first.token == second.token {
		t.Error("two startups produced the same token")
	}
}

// ::1 and 127.0.0.2 are loopback addresses, so the server keeps the safe
// defaults there, but the default list names only localhost and 127.0.0.1. The
// address the server is bound to is its own address: the URL it prints for it
// must be served, not refused by the host check.
func TestLoopbackListenerAllowsItsOwnAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ip := range []string{"127.0.0.1", "::1", "127.0.0.2"} {
		t.Run(ip, func(t *testing.T) {
			access := mustAccess(t, ip, false)
			router := gin.New()
			router.Use(access.middleware(cors.NewChecker(access.withOwnAddress(config.DefaultCORSOrigins)))...)
			router.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "page") })
			router.POST("/api/graphql", func(c *gin.Context) { c.String(http.StatusOK, "ran") })

			host := net.JoinHostPort(ip, "8080")
			if rec := guardedRequest(router, "GET", host, "", "/"); rec.Code != 200 {
				t.Errorf("GET under its own address %s: status %d (%s), want 200", host, rec.Code, rec.Body.String())
			}
			if rec := guardedRequest(router, "POST", host, "http://"+host, "/api/graphql"); rec.Code != 200 {
				t.Errorf("POST from its own page %s: status %d (%s), want 200", host, rec.Code, rec.Body.String())
			}
			// Another port of the address, a dev server of the UI, is listed like it
			// is for 127.0.0.1 in the default list.
			if rec := guardedRequest(router, "POST", host, "http://"+net.JoinHostPort(ip, "5173"), "/api/graphql"); rec.Code != 200 {
				t.Errorf("POST from another port of %s: status %d (%s), want 200", ip, rec.Code, rec.Body.String())
			}
			// Nothing else gets in.
			if rec := guardedRequest(router, "GET", "evil.example:8080", "", "/"); rec.Code != 403 {
				t.Errorf("GET under a foreign host: status %d, want 403", rec.Code)
			}
			if rec := guardedRequest(router, "POST", host, "http://evil.example", "/api/graphql"); rec.Code != 403 {
				t.Errorf("POST from a foreign origin: status %d, want 403", rec.Code)
			}
		})
	}
}

// The list the operator gave is what the server prints and uses: the own
// address is added only where it is missing, and the list passed in, which may
// be config.DefaultCORSOrigins itself, is never modified.
func TestWithOwnAddressOnlyAddsWhatIsMissing(t *testing.T) {
	defaults := slices.Clone(config.DefaultCORSOrigins)

	if got := mustAccess(t, "127.0.0.1", false).withOwnAddress(config.DefaultCORSOrigins); !slices.Equal(got, defaults) {
		t.Errorf("127.0.0.1 is in the default list already: got %v", got)
	}
	if got := mustAccess(t, "0.0.0.0", false).withOwnAddress(config.DefaultCORSOrigins); !slices.Equal(got, defaults) {
		t.Errorf("a listener on the network has no loopback address of its own: got %v", got)
	}
	if got := mustAccess(t, "192.168.1.50", false).withOwnAddress(config.DefaultCORSOrigins); !slices.Equal(got, defaults) {
		t.Errorf("a LAN address is not added: got %v", got)
	}

	got := mustAccess(t, "::1", false).withOwnAddress(config.DefaultCORSOrigins)
	if want := append(slices.Clone(defaults), "http://[::1]:*"); !slices.Equal(got, want) {
		t.Errorf("::1: got %v, want %v", got, want)
	}
	got = mustAccess(t, "127.0.0.1", false).withOwnAddress([]string{"http://localhost:5173"})
	if want := []string{"http://localhost:5173", "http://127.0.0.1:*"}; !slices.Equal(got, want) {
		t.Errorf("a replaced list keeps the own address: got %v, want %v", got, want)
	}
	if !slices.Equal(config.DefaultCORSOrigins, defaults) {
		t.Errorf("the default list was modified: %v", config.DefaultCORSOrigins)
	}
}

// Behind a proxy the browser reaches the server under the public name. That
// host passes the host check and a page of that origin may change things
// without a --cors-origin entry, whichever Host the proxy forwards. Every
// other host and origin stays out.
func TestPublicURLAllowsItsOwnHostAndOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	access := mustAccessOpts(t, "127.0.0.1", serveOptions{publicURL: mustPublicURL(t, "https://box.example.ts.net/lif/")})

	defaults := slices.Clone(config.DefaultCORSOrigins)
	origins := access.withPublicOrigin(access.withOwnAddress(config.DefaultCORSOrigins))
	if want := append(slices.Clone(defaults), "https://box.example.ts.net"); !slices.Equal(origins, want) {
		t.Fatalf("origins = %v, want %v", origins, want)
	}
	if !slices.Equal(config.DefaultCORSOrigins, defaults) {
		t.Errorf("the default list was modified: %v", config.DefaultCORSOrigins)
	}

	router := gin.New()
	router.Use(access.middleware(cors.NewChecker(origins))...)
	ran := 0
	router.POST("/api/graphql", func(c *gin.Context) { ran++; c.String(http.StatusOK, "ran") })

	tests := []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{"the UI under the public name", "box.example.ts.net", "https://box.example.ts.net", 200},
		{"a proxy that forwards its own Host", "127.0.0.1:8080", "https://box.example.ts.net", 200},
		{"a script through the proxy", "box.example.ts.net", "", 200},
		{"a page of another site", "box.example.ts.net", "https://evil.example", 403},
		{"a foreign host", "evil.example", "https://evil.example", 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := ran
			rec := guardedRequest(router, "POST", tt.host, tt.origin, "/api/graphql")
			if rec.Code != tt.want {
				t.Fatalf("host=%q origin=%q: status %d, want %d (body %q)", tt.host, tt.origin, rec.Code, tt.want, rec.Body.String())
			}
			if tt.want == 403 && ran != before {
				t.Errorf("the refused request still reached the handler")
			}
		})
	}
}

// The playground page lives under the prefix of the public URL, so it gets the
// API as an absolute URL; a root-relative path would leave the prefix.
func TestGraphQLEndpointFollowsThePublicURL(t *testing.T) {
	if got := mustAccess(t, "127.0.0.1", false).graphqlEndpoint(); got != "/api/graphql" {
		t.Errorf("without a public URL: %q, want /api/graphql", got)
	}
	a := mustAccessOpts(t, "127.0.0.1", serveOptions{publicURL: mustPublicURL(t, "https://box.example.ts.net/lif/")})
	if got, want := a.graphqlEndpoint(), "https://box.example.ts.net/lif/api/graphql"; got != want {
		t.Errorf("behind a public URL: %q, want %q", got, want)
	}
}

func TestParsePublicURL(t *testing.T) {
	valid := map[string]string{
		"https://box.example.ts.net":         "https://box.example.ts.net",
		"https://box.example.ts.net/lif/":    "https://box.example.ts.net/lif",
		"https://Box.Example.TS.net/lif//":   "https://box.example.ts.net/lif",
		"http://nas.local:8080":              "http://nas.local:8080",
		"https://box.example.ts.net:443/lif": "https://box.example.ts.net/lif",
		"http://nas.local:80":                "http://nas.local",
	}
	for in, want := range valid {
		u, err := parsePublicURL(in)
		if err != nil {
			t.Errorf("parsePublicURL(%q): %v", in, err)
			continue
		}
		if got := u.String(); got != want {
			t.Errorf("parsePublicURL(%q) = %q, want %q", in, got, want)
		}
	}

	for _, in := range []string{"", "box.example.ts.net", "/lif", "ftp://box.example", "https://", "https:box", "https://box?x=1", "https://box/?", "https://box#top"} {
		_, err := parsePublicURL(in)
		if err == nil || !strings.Contains(err.Error(), "must be an absolute http:// or https:// URL without query or fragment") {
			t.Errorf("parsePublicURL(%q): err = %v, want the invalid --public-url error", in, err)
		}
	}
}

// On loopback the Host and Origin checks stand in front of every route. A
// browser page that is not the UI must not change anything, and neither must a
// page under a name that resolves to this machine (DNS rebinding). A client
// that sends no Origin (curl, a script) keeps working: the documented curl
// example depends on it.
func TestLoopbackGuardRefusesForeignHostAndOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mustAccess(t, "127.0.0.1", false).middleware(cors.NewChecker(config.DefaultCORSOrigins))...)
	ran := 0
	router.POST("/api/graphql", func(c *gin.Context) { ran++; c.String(http.StatusOK, "ran") })
	router.GET("/api/graphql", func(c *gin.Context) { c.String(http.StatusOK, "read") })

	tests := []struct {
		name   string
		method string
		host   string
		origin string
		want   int
	}{
		{"the UI itself", "POST", "localhost:8080", "http://localhost:8080", 200},
		{"the dev server of the UI", "POST", "localhost:8080", "http://localhost:5173", 200},
		{"curl by name", "POST", "localhost:8080", "", 200},
		{"curl by address", "POST", "127.0.0.1:8080", "", 200},
		{"a page of another site", "POST", "localhost:8080", "http://evil.example", 403},
		{"a page of another site on a local port it does not own", "POST", "localhost:8080", "http://evil.example:5173", 403},
		{"a sandboxed page", "POST", "localhost:8080", "null", 403},
		{"DNS rebinding", "POST", "evil.example:8080", "http://evil.example:8080", 403},
		{"DNS rebinding without an Origin", "POST", "evil.example:8080", "", 403},
		{"the LAN address", "POST", "192.168.1.50:8080", "", 403},
		{"a read under a rebound name", "GET", "evil.example:8080", "", 403},
		{"a read by name", "GET", "localhost:8080", "", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := ran
			rec := guardedRequest(router, tt.method, tt.host, tt.origin, "/api/graphql")
			if rec.Code != tt.want {
				t.Fatalf("%s host=%q origin=%q: status %d, want %d (body %q)", tt.method, tt.host, tt.origin, rec.Code, tt.want, rec.Body.String())
			}
			if tt.want == 403 && ran != before {
				t.Errorf("the refused request still reached the handler")
			}
		})
	}
}

// The origin list names the pages that may use the server besides its own UI:
// --cors-origin replaces the default, so the dev server of the UI is refused
// when it is not listed. The page the server itself serves is of its own origin
// and needs no entry.
func TestGuardFollowsTheConfiguredOriginList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mustAccess(t, "127.0.0.1", false).middleware(cors.NewChecker([]string{"http://localhost:5173"}))...)
	router.POST("/api/graphql", func(c *gin.Context) { c.String(http.StatusOK, "ran") })

	if rec := guardedRequest(router, "POST", "localhost:8080", "http://localhost:5173", "/api/graphql"); rec.Code != 200 {
		t.Errorf("the listed origin: status %d, want 200", rec.Code)
	}
	if rec := guardedRequest(router, "POST", "localhost:8080", "http://localhost:8080", "/api/graphql"); rec.Code != 200 {
		t.Errorf("the UI of the server itself: status %d, want 200", rec.Code)
	}
	if rec := guardedRequest(router, "POST", "localhost:8080", "http://localhost:9999", "/api/graphql"); rec.Code != 403 {
		t.Errorf("another local port outside the replaced list: status %d, want 403", rec.Code)
	}
}

// --cors-origin '*' lets any page read the answers (the CORS header, tested in
// package cors). It does not switch off the host check, does not let a page of
// another site change anything, and does not open the terminal: otherwise one
// line in a committed .lif.yml would give every website a shell, through the
// terminal WebSocket or through the writeTerminalInput mutation.
func TestWildcardOriginIsForTheCORSHeaderOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mustAccess(t, "127.0.0.1", false).middleware(cors.NewChecker([]string{"*"}))...)
	ran := 0
	router.POST("/api/graphql", func(c *gin.Context) { ran++; c.String(http.StatusOK, "ran") })
	router.GET("/api/graphql", func(c *gin.Context) { c.String(http.StatusOK, "read") })

	tests := []struct {
		name   string
		method string
		host   string
		origin string
		want   int
	}{
		{"the UI by name", "POST", "localhost:8080", "http://localhost:8080", 200},
		{"the dev server of the UI", "POST", "localhost:8080", "http://localhost:5173", 200},
		{"a page of another site changes nothing", "POST", "localhost:8080", "http://evil.example", 403},
		{"a sandboxed page changes nothing", "POST", "localhost:8080", "null", 403},
		{"a page of another site may read", "GET", "localhost:8080", "http://evil.example", 200},
		{"DNS rebinding is still refused", "POST", "evil.example:8080", "http://evil.example:8080", 403},
		{"DNS rebinding of a read is still refused", "GET", "evil.example:8080", "", 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := ran
			rec := guardedRequest(router, tt.method, tt.host, tt.origin, "/api/graphql")
			if rec.Code != tt.want {
				t.Fatalf("%s host=%q origin=%q: status %d, want %d (body %q)", tt.method, tt.host, tt.origin, rec.Code, tt.want, rec.Body.String())
			}
			if tt.want == 403 && ran != before {
				t.Errorf("the refused request still reached the handler")
			}
		})
	}
}

// terminalServer serves /api/terminal behind the guards of access and the
// given origin list, with a real shell behind it.
func terminalServer(t *testing.T, access serveAccess, origins []string) *httptest.Server {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	checker := cors.NewChecker(origins)
	router := gin.New()
	router.Use(access.middleware(checker)...)
	termMgr := terminal.NewManager(nil)
	t.Cleanup(termMgr.Shutdown)
	RegisterTerminalRoute(router, termMgr, nil, checker.CheckOriginFunc(), t.TempDir())

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

// dialTerminal opens the terminal WebSocket the way a client with the given
// headers would. The returned response carries the refusal when err != nil.
func dialTerminal(t *testing.T, srv *httptest.Server, host string, header http.Header) (*coderws.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return coderws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/terminal", &coderws.DialOptions{
		HTTPHeader: header,
		Host:       host,
	})
}

// A shell needs a browser that identifies itself with an allowed Origin, on a
// Host the server answers to. A client without an Origin gets none: only the
// web UI uses the terminal, and a browser always sends one on a WebSocket
// upgrade.
func TestTerminalNeedsAnAllowedOriginAndHost(t *testing.T) {
	srv := terminalServer(t, mustAccess(t, "127.0.0.1", false), config.DefaultCORSOrigins)
	local := srv.Listener.Addr().String() // 127.0.0.1:<port>

	refused := []struct {
		name   string
		host   string
		header http.Header
	}{
		{"no Origin (a script)", local, http.Header{}},
		{"a page of another site", local, http.Header{"Origin": {"http://evil.example"}}},
		{"a sandboxed page", local, http.Header{"Origin": {"null"}}},
		{"DNS rebinding", "evil.example:8080", http.Header{"Origin": {"http://evil.example:8080"}}},
		{"an allowed Origin under a foreign Host", "evil.example:8080", http.Header{"Origin": {"http://localhost:5173"}}},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			conn, resp, err := dialTerminal(t, srv, tt.host, tt.header)
			if err == nil {
				conn.CloseNow()
				t.Fatal("the terminal upgrade was accepted")
			}
			if resp == nil || resp.StatusCode != http.StatusForbidden {
				t.Fatalf("got %v, want a 403 response", resp)
			}
		})
	}

	t.Run("the UI gets a working shell", func(t *testing.T) {
		conn, _, err := dialTerminal(t, srv, local, http.Header{"Origin": {"http://localhost:5173"}})
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.CloseNow()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := conn.Write(ctx, coderws.MessageText, []byte(`{"type":"init","sessionId":"__central__","cols":80,"rows":24}`)); err != nil {
			t.Fatal(err)
		}
		if err := conn.Write(ctx, coderws.MessageText, []byte(`{"type":"input","data":"echo lif-$((40+2))\n"}`)); err != nil {
			t.Fatal(err)
		}
		var seen strings.Builder
		for !strings.Contains(seen.String(), "lif-42") {
			_, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("no shell output with lif-42, got %q: %v", seen.String(), err)
			}
			seen.Write(data)
		}
	})
}

// --cors-origin '*' must not hand a shell to every website: the terminal
// WebSocket still needs an Origin that is listed or the server's own, and the
// host check stays on. The UI of the server itself keeps its terminal.
func TestWildcardDoesNotOpenTheTerminal(t *testing.T) {
	srv := terminalServer(t, mustAccess(t, "127.0.0.1", false), []string{"*"})
	local := srv.Listener.Addr().String()

	refused := []struct {
		name   string
		host   string
		header http.Header
	}{
		{"a page of another site", local, http.Header{"Origin": {"http://evil.example"}}},
		{"a sandboxed page", local, http.Header{"Origin": {"null"}}},
		{"no Origin (a script)", local, http.Header{}},
		{"DNS rebinding", "evil.example:8080", http.Header{"Origin": {"http://evil.example:8080"}}},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			conn, resp, err := dialTerminal(t, srv, tt.host, tt.header)
			if err == nil {
				conn.CloseNow()
				t.Fatal("the terminal upgrade was accepted under *")
			}
			if resp == nil || resp.StatusCode != http.StatusForbidden {
				t.Fatalf("got %v, want a 403 response", resp)
			}
		})
	}

	t.Run("the UI of the server itself", func(t *testing.T) {
		conn, _, err := dialTerminal(t, srv, local, http.Header{"Origin": {"http://" + local}})
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conn.CloseNow()
	})
}

// Without the opt-in the terminal route answers with a reason, not with the
// single-page app, which would turn the upgrade into a confusing 200.
func TestDisabledTerminalRouteSaysWhy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerTerminalDisabled(router)

	rec := guardedRequest(router, "GET", "localhost:8080", "http://localhost:8080", "/api/terminal")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--allow-remote-terminal") {
		t.Errorf("the refusal does not name the opt-in flag: %q", rec.Body.String())
	}
}

// Off loopback, Host and Origin prove nothing against a script. The server
// asks every client for the token instead: in a header for scripts, in a
// cookie for the browser after it opened the printed address once.
func TestNetworkListenerRequiresTheToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	access := mustAccess(t, "0.0.0.0", false)
	router := gin.New()
	router.Use(access.middleware(cors.NewChecker(config.DefaultCORSOrigins))...)
	ran := 0
	router.POST("/api/graphql", func(c *gin.Context) { ran++; c.String(http.StatusOK, "ran") })
	router.GET("/page", func(c *gin.Context) { c.String(http.StatusOK, "page") })

	do := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	get := func(target string, mod func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "http://192.168.1.50:8080"+target, nil)
		if mod != nil {
			mod(req)
		}
		return do(req)
	}

	t.Run("no credential", func(t *testing.T) {
		if rec := get("/page", nil); rec.Code != 401 || rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("status %d, WWW-Authenticate %q; want 401 with a challenge", rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
		before := ran
		if rec := guardedRequest(router, "POST", "192.168.1.50:8080", "", "/api/graphql"); rec.Code != 401 || ran != before {
			t.Errorf("mutation without token: status %d, handler ran %v", rec.Code, ran != before)
		}
	})

	t.Run("wrong credentials", func(t *testing.T) {
		for name, mod := range map[string]func(*http.Request){
			"bearer":          func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") },
			"cookie":          func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "lif_token_8080", Value: "nope"}) },
			"query":           func(r *http.Request) { r.URL.RawQuery = "token=nope" },
			"another port":    func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "lif_token_9090", Value: access.token}) },
			"empty bearer":    func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") },
			"token as scheme": func(r *http.Request) { r.Header.Set("Authorization", access.token) },
		} {
			if rec := get("/page", mod); rec.Code != 401 {
				t.Errorf("%s: status %d, want 401", name, rec.Code)
			}
		}
	})

	t.Run("bearer token", func(t *testing.T) {
		if rec := get("/page", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+access.token) }); rec.Code != 200 {
			t.Errorf("status %d, want 200", rec.Code)
		}
		req := httptest.NewRequest("POST", "http://192.168.1.50:8080/api/graphql", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+access.token)
		if rec := do(req); rec.Code != 200 {
			t.Errorf("mutation with token: status %d, want 200", rec.Code)
		}
	})

	t.Run("printed address sets the cookie and hides the token", func(t *testing.T) {
		rec := get("/page?item=abc&token="+access.token, nil)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status %d, want 303", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/page?item=abc" {
			t.Errorf("Location %q, want /page?item=abc", loc)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("got %d cookies, want 1", len(cookies))
		}
		c := cookies[0]
		if c.Name != "lif_token_8080" || c.Value != access.token || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
			t.Errorf("cookie %+v: want name lif_token_8080, the token, HttpOnly, SameSite=Lax, Path=/", c)
		}
		if rec := get("/page", func(r *http.Request) { r.AddCookie(c) }); rec.Code != 200 {
			t.Errorf("with the cookie: status %d, want 200", rec.Code)
		}
	})

	t.Run("the redirect stays on this site", func(t *testing.T) {
		rec := get("//evil.example/page?token="+access.token, nil)
		loc := rec.Header().Get("Location")
		if rec.Code != http.StatusSeeOther || strings.HasPrefix(loc, "//") {
			t.Errorf("status %d, Location %q: want a 303 to a path on this server", rec.Code, loc)
		}
	})

	t.Run("a query token on a mutation does not redirect", func(t *testing.T) {
		req := httptest.NewRequest("POST", "http://192.168.1.50:8080/api/graphql?token="+access.token, strings.NewReader("{}"))
		if rec := do(req); rec.Code != 200 {
			t.Errorf("status %d, want 200", rec.Code)
		}
	})

	t.Run("the origin check still applies", func(t *testing.T) {
		req := httptest.NewRequest("POST", "http://192.168.1.50:8080/api/graphql", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+access.token)
		req.Header.Set("Origin", "http://evil.example")
		before := ran
		if rec := do(req); rec.Code != 403 || ran != before {
			t.Errorf("status %d, handler ran %v: want a 403 for a foreign page that has the token", rec.Code, ran != before)
		}
	})

	// The UI the server serves is of the server's own origin, so it needs no
	// entry in the origin list, whatever address the operator reaches it under.
	t.Run("the page the server serves needs no entry in the origin list", func(t *testing.T) {
		for _, tt := range []struct {
			host, origin string
			want         int
		}{
			{"192.168.1.50:8080", "http://192.168.1.50:8080", 200},
			{"nas.local:8080", "http://nas.local:8080", 200},
			{"192.168.1.50:8080", "http://192.168.1.50:9999", 403}, // another page on the same host
			{"192.168.1.50:8080", "https://192.168.1.50:8080", 403},
			{"192.168.1.50:8080", "http://192.168.1.51:8080", 403},
		} {
			req := httptest.NewRequest("POST", "http://"+tt.host+"/api/graphql", strings.NewReader("{}"))
			req.Header.Set("Authorization", "Bearer "+access.token)
			req.Header.Set("Origin", tt.origin)
			if rec := do(req); rec.Code != tt.want {
				t.Errorf("Host %q, Origin %q: status %d, want %d", tt.host, tt.origin, rec.Code, tt.want)
			}
		}
	})

	// A page of another origin cannot use a server on the network, and
	// nothing here is meant to change that: the preflight carries no
	// credentials and gets 401, and the browser sends neither the cookie nor an
	// Authorization header across origins. Only the UI the server serves works.
	t.Run("a preflight needs the token too", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "http://192.168.1.50:8080/api/graphql", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("Access-Control-Request-Method", "POST")
		if rec := do(req); rec.Code != 401 {
			t.Errorf("preflight without token: status %d, want 401", rec.Code)
		}
	})

	t.Run("any host name works", func(t *testing.T) {
		// The token, not the Host header, protects a listener on the network:
		// the operator reaches it under a name nobody can list in advance.
		for _, host := range []string{"192.168.1.50:8080", "nas.local:8080", "localhost:8080"} {
			req := httptest.NewRequest("GET", "http://"+host+"/page", nil)
			req.Header.Set("Authorization", "Bearer "+access.token)
			if rec := do(req); rec.Code != 200 {
				t.Errorf("host %q: status %d, want 200", host, rec.Code)
			}
		}
	})
}

// The access token travels in ?token=, and a log is no place for it: the
// access log leaves the query string out.
func TestAccessLogOmitsTheQueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	router := gin.New()
	router.Use(accessLog(&buf))
	router.GET("/page", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "http://localhost:8080/page?item=abc&token=SECRET-TOKEN", nil))

	got := buf.String()
	if strings.Contains(got, "SECRET-TOKEN") {
		t.Errorf("the access log carries the token: %q", got)
	}
	if !strings.Contains(got, `"/page"`) {
		t.Errorf("the access log lost the path: %q", got)
	}
}

// A listener on all interfaces is shown under the first LAN address of the
// machine, the one a person on the network can open.
func TestFirstGlobalUnicastIPv4(t *testing.T) {
	ipnet := func(cidr string) net.Addr {
		ip, n, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}
		n.IP = ip
		return n
	}
	tests := []struct {
		name  string
		addrs []net.Addr
		want  string
	}{
		{"skips loopback, link-local and IPv6", []net.Addr{ipnet("127.0.0.1/8"), ipnet("169.254.3.4/16"), ipnet("fe80::1/64"), ipnet("2001:db8::1/64"), ipnet("192.168.1.50/24")}, "192.168.1.50"},
		{"the first one wins", []net.Addr{ipnet("10.0.0.7/8"), ipnet("192.168.1.50/24")}, "10.0.0.7"},
		{"a VPN address counts", []net.Addr{ipnet("100.85.223.39/32")}, "100.85.223.39"},
		{"none on the machine", []net.Addr{ipnet("127.0.0.1/8"), ipnet("fe80::1/64")}, ""},
		{"no addresses", nil, ""},
		{"not an IP network", []net.Addr{&net.TCPAddr{IP: net.ParseIP("192.168.1.50"), Port: 1}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstGlobalUnicastIPv4(tt.addrs); got != tt.want {
				t.Errorf("firstGlobalUnicastIPv4() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The startup log says what the server really listens on and what that means.
func TestStartupLinesNameTheRealBindAddress(t *testing.T) {
	// The addresses of the machine the log is written on: loopback and
	// link-local ones never count, the first IPv4 LAN address is the one to open.
	machine := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("192.168.1.50"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("10.0.0.7"), Mask: net.CIDRMask(8, 32)},
	}
	join := func(a serveAccess, origins []string) string {
		return strings.Join(a.startupLines(origins, machine), "\n")
	}
	mustContain := func(t *testing.T, log string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !strings.Contains(log, want) {
				t.Errorf("log lacks %q:\n%s", want, log)
			}
		}
	}
	mustNotContain := func(t *testing.T, log string, bans ...string) {
		t.Helper()
		for _, ban := range bans {
			if strings.Contains(log, ban) {
				t.Errorf("log contains %q:\n%s", ban, log)
			}
		}
	}

	t.Run("loopback", func(t *testing.T) {
		log := join(mustAccess(t, "127.0.0.1", false), config.DefaultCORSOrigins)
		mustContain(t, log,
			"[lif] Listening on 127.0.0.1:8080 (loopback only",
			"[lif] Starting server at http://127.0.0.1:8080/",
			"[lif] GraphQL Playground: http://127.0.0.1:8080/playground",
			"[lif] Allowed origins: http://localhost:*, http://127.0.0.1:*",
			"[lif] Terminal: enabled",
		)
		mustNotContain(t, log, "token", "WARNING", "localhost:8080", "Open the UI")
	})

	t.Run("IPv6 loopback", func(t *testing.T) {
		mustContain(t, join(mustAccess(t, "::1", false), append(config.DefaultCORSOrigins, "http://[::1]:*")),
			"[lif] Listening on [::1]:8080 (loopback only",
			"[lif] Starting server at http://[::1]:8080/",
			"[lif] Allowed origins: http://localhost:*, http://127.0.0.1:*, http://[::1]:*",
		)
	})

	// Every request needs the token there, so the lines name an address that
	// works for a person on the network, with the token in it, and not the
	// address the server cannot be opened under without it.
	t.Run("all interfaces", func(t *testing.T) {
		a := mustAccess(t, "0.0.0.0", false)
		log := join(a, config.DefaultCORSOrigins)
		mustContain(t, log,
			"[lif] Listening on 0.0.0.0:8080 (ALL network interfaces",
			"[lif] WARNING:",
			"[lif] Access token: "+a.token,
			"[lif] Open the UI: http://192.168.1.50:8080/?token="+a.token,
			"[lif] GraphQL Playground: http://192.168.1.50:8080/playground?token="+a.token,
			"[lif] Terminal: disabled",
			"--allow-remote-terminal",
		)
		mustNotContain(t, log, "Starting server at", "--cors-origin", "<address", "localhost:8080")
	})

	t.Run("all interfaces on a machine without a LAN address", func(t *testing.T) {
		a := mustAccess(t, "0.0.0.0", false)
		log := strings.Join(a.startupLines(config.DefaultCORSOrigins, machine[:2]), "\n")
		mustContain(t, log, "[lif] Open the UI: http://localhost:8080/?token="+a.token)
	})

	t.Run("one LAN address", func(t *testing.T) {
		a := mustAccess(t, "10.0.0.9", false)
		log := join(a, config.DefaultCORSOrigins)
		mustContain(t, log,
			"[lif] Listening on 10.0.0.9:8080 (non-loopback address",
			"[lif] Open the UI: http://10.0.0.9:8080/?token="+a.token,
		)
		mustNotContain(t, log, "Starting server at", "--cors-origin", "192.168.1.50")
	})

	t.Run("terminal opted in on the network", func(t *testing.T) {
		log := join(mustAccess(t, "0.0.0.0", true), config.DefaultCORSOrigins)
		mustContain(t, log, "[lif] Terminal: enabled (--allow-remote-terminal)", "shell")
		mustNotContain(t, log, "Terminal: disabled")
	})

	t.Run("public URL", func(t *testing.T) {
		a := mustAccessOpts(t, "127.0.0.1", serveOptions{publicURL: mustPublicURL(t, "https://box.example.ts.net/lif")})
		log := join(a, config.DefaultCORSOrigins)
		mustContain(t, log,
			"[lif] Listening on 127.0.0.1:8080 (loopback only",
			"[lif] Starting server at https://box.example.ts.net/lif/",
			"[lif] GraphQL Playground: https://box.example.ts.net/lif/playground",
		)
		mustNotContain(t, log, "127.0.0.1:8080/")
	})

	t.Run("wildcard origin", func(t *testing.T) {
		log := join(mustAccess(t, "127.0.0.1", false), []string{"*"})
		mustContain(t, log, "[lif] WARNING: allowed origins include *", "shell")
		mustNotContain(t, log, "checks are off")
	})
}
