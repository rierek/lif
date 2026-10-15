package commands

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"

	"github.com/rierek/lif/internal/cors"
	"github.com/rierek/lif/internal/terminal"
)

func tcpAddr(ip string, port int) *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
}

func mustAccess(t *testing.T, ip string, allowRemoteTerminal bool) serveAccess {
	t.Helper()
	a, err := newServeAccess(tcpAddr(ip, 8080), allowRemoteTerminal)
	if err != nil {
		t.Fatalf("newServeAccess(%s): %v", ip, err)
	}
	return a
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

// On loopback the Host and Origin checks stand in front of every route. A
// browser page that is not the UI must not change anything, and neither must a
// page under a name that resolves to this machine (DNS rebinding). A client
// that sends no Origin (curl, a script) keeps working: the documented curl
// example depends on it.
func TestLoopbackGuardRefusesForeignHostAndOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mustAccess(t, "127.0.0.1", false).middleware(cors.NewChecker(cors.DefaultOrigins))...)
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

// The origin list is the whole list: --cors-origin replaces the default, so a
// page of the UI itself at an address that is not listed is refused too.
func TestGuardFollowsTheConfiguredOriginList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mustAccess(t, "127.0.0.1", false).middleware(cors.NewChecker([]string{"http://localhost:5173"}))...)
	router.POST("/api/graphql", func(c *gin.Context) { c.String(http.StatusOK, "ran") })

	if rec := guardedRequest(router, "POST", "localhost:8080", "http://localhost:5173", "/api/graphql"); rec.Code != 200 {
		t.Errorf("the listed origin: status %d, want 200", rec.Code)
	}
	if rec := guardedRequest(router, "POST", "localhost:8080", "http://localhost:8080", "/api/graphql"); rec.Code != 403 {
		t.Errorf("an origin outside the replaced list: status %d, want 403", rec.Code)
	}
}

// --cors-origin '*' is the operator's decision to switch the checks off.
func TestGuardLetsEveryOriginPassForTheWildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mustAccess(t, "127.0.0.1", false).middleware(cors.NewChecker([]string{"*"}))...)
	router.POST("/api/graphql", func(c *gin.Context) { c.String(http.StatusOK, "ran") })

	if rec := guardedRequest(router, "POST", "evil.example:8080", "http://evil.example:8080", "/api/graphql"); rec.Code != 200 {
		t.Errorf("status %d, want 200 under the wildcard", rec.Code)
	}
}

// terminalServer serves /api/terminal behind the guards of access, with a real
// shell behind it.
func terminalServer(t *testing.T, access serveAccess) *httptest.Server {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	checker := cors.NewChecker(cors.DefaultOrigins)
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
	srv := terminalServer(t, mustAccess(t, "127.0.0.1", false))
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
	router.Use(access.middleware(cors.NewChecker(cors.DefaultOrigins))...)
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

// The startup log says what the server really listens on and what that means.
func TestStartupLinesNameTheRealBindAddress(t *testing.T) {
	join := func(a serveAccess, origins []string) string {
		return strings.Join(a.startupLines(origins), "\n")
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
		log := join(mustAccess(t, "127.0.0.1", false), cors.DefaultOrigins)
		mustContain(t, log,
			"[lif] Listening on 127.0.0.1:8080 (loopback only",
			"[lif] Starting server at http://127.0.0.1:8080/",
			"[lif] GraphQL Playground: http://127.0.0.1:8080/playground",
			"[lif] Allowed origins: http://localhost:*, http://127.0.0.1:*",
			"[lif] Terminal: enabled",
		)
		mustNotContain(t, log, "token", "WARNING", "localhost:8080")
	})

	t.Run("IPv6 loopback", func(t *testing.T) {
		mustContain(t, join(mustAccess(t, "::1", false), cors.DefaultOrigins),
			"[lif] Listening on [::1]:8080 (loopback only",
			"[lif] Starting server at http://[::1]:8080/",
		)
	})

	t.Run("all interfaces", func(t *testing.T) {
		a := mustAccess(t, "0.0.0.0", false)
		log := join(a, cors.DefaultOrigins)
		mustContain(t, log,
			"[lif] Listening on 0.0.0.0:8080 (ALL network interfaces",
			"[lif] WARNING:",
			"[lif] Access token: "+a.token,
			"?token="+a.token,
			"--cors-origin",
			"[lif] Starting server at http://localhost:8080/",
			"[lif] Terminal: disabled",
			"--allow-remote-terminal",
		)
	})

	t.Run("one LAN address", func(t *testing.T) {
		a := mustAccess(t, "192.168.1.50", false)
		mustContain(t, join(a, cors.DefaultOrigins),
			"[lif] Listening on 192.168.1.50:8080 (non-loopback address",
			"http://192.168.1.50:8080/?token="+a.token,
			"--cors-origin 'http://192.168.1.50:*'",
			"[lif] Starting server at http://192.168.1.50:8080/",
		)
	})

	t.Run("terminal opted in on the network", func(t *testing.T) {
		log := join(mustAccess(t, "0.0.0.0", true), cors.DefaultOrigins)
		mustContain(t, log, "[lif] Terminal: enabled (--allow-remote-terminal)", "shell")
		mustNotContain(t, log, "Terminal: disabled")
	})

	t.Run("wildcard origin", func(t *testing.T) {
		mustContain(t, join(mustAccess(t, "127.0.0.1", false), []string{"*"}),
			"[lif] WARNING: allowed origins include *",
		)
	})
}
