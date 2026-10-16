package commands

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestMaxBodyMiddlewareCapsRequestBody pins the request body limit. Without it
// a single POST can make the server allocate as much memory as the client
// cares to send.
func TestMaxBodyMiddlewareCapsRequestBody(t *testing.T) {
	router := gin.New()
	router.Use(maxBodyMiddleware(1024))

	var readErr error
	router.POST("/api/graphql", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(http.StatusOK)
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/graphql", "application/json", strings.NewReader(strings.Repeat("x", 4096)))
	if err == nil {
		resp.Body.Close()
	}

	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Fatalf("reading an oversized body returned %v, want a *http.MaxBytesError", readErr)
	}
}

// TestMaxBodyMiddlewareLetsSmallBodiesThrough guards the other direction: the
// limit must not truncate ordinary GraphQL traffic.
func TestMaxBodyMiddlewareLetsSmallBodiesThrough(t *testing.T) {
	router := gin.New()
	router.Use(maxBodyMiddleware(1024))

	var got string
	router.POST("/api/graphql", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Errorf("reading a small body: %v", err)
		}
		got = string(body)
		c.Status(http.StatusOK)
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	want := strings.Repeat("y", 512)
	resp, err := http.Post(srv.URL+"/api/graphql", "application/json", strings.NewReader(want))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if got != want {
		t.Fatalf("handler read %d bytes, want %d", len(got), len(want))
	}
}

// wsTestServer wires both upgrade endpoints behind one shared limiter, the way
// serve.go does, and holds every accepted connection open until the test
// releases it.
func wsTestServer(t *testing.T, limiter *connLimiter) (*httptest.Server, func()) {
	t.Helper()

	release := make(chan struct{})
	var once sync.Once
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	hold := func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-release
	}

	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/api/graphql", hold)
	router.GET("/api/terminal", hold)

	srv := httptest.NewServer(router)
	return srv, func() {
		once.Do(func() { close(release) })
		srv.Close()
	}
}

func dialWS(t *testing.T, srv *httptest.Server, path string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	return dialer.Dial(url, nil)
}

// TestConnLimiterRejectsAboveCap is the file-descriptor guard: a client that
// opens subscriptions in a loop exhausts the process's descriptors, and the
// cap has to hold across both upgrade endpoints, not per route.
func TestConnLimiterRejectsAboveCap(t *testing.T) {
	limiter := newConnLimiter(2)
	srv, done := wsTestServer(t, limiter)
	defer done()

	first, _, err := dialWS(t, srv, "/api/graphql")
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	defer first.Close()

	second, _, err := dialWS(t, srv, "/api/terminal")
	if err != nil {
		t.Fatalf("second dial (other endpoint): %v", err)
	}
	defer second.Close()

	third, resp, err := dialWS(t, srv, "/api/graphql")
	if err == nil {
		third.Close()
		t.Fatal("third dial was accepted above the cap of 2")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("rejected dial answered with %v, want %d", resp, http.StatusServiceUnavailable)
	}
}

// TestConnLimiterReleasesOnClose pins the decrement: a limiter that only counts
// up turns a temporary burst into a permanently dead endpoint.
func TestConnLimiterReleasesOnClose(t *testing.T) {
	limiter := newConnLimiter(1)

	release := make(chan struct{})
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/api/terminal", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-release
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	first, _, err := dialWS(t, srv, "/api/terminal")
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	close(release)
	first.Close()

	// The handler returns asynchronously; give it a moment to unwind.
	var lastErr error
	for i := 0; i < 50; i++ {
		conn, _, err := dialWS(t, srv, "/api/terminal")
		if err == nil {
			conn.Close()
			return
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("slot was never released: %v", lastErr)
}

// TestConnLimiterIgnoresPlainRequests keeps ordinary GraphQL POSTs out of the
// count — they finish in milliseconds and must not consume a subscription slot.
func TestConnLimiterIgnoresPlainRequests(t *testing.T) {
	limiter := newConnLimiter(1)

	router := gin.New()
	router.Use(limiter.Middleware())
	router.POST("/api/graphql", func(c *gin.Context) { c.Status(http.StatusOK) })

	srv := httptest.NewServer(router)
	defer srv.Close()

	for i := 0; i < 3; i++ {
		resp, err := http.Post(srv.URL+"/api/graphql", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("POST %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST %d answered %d, want 200", i, resp.StatusCode)
		}
	}
}

// TestNewHTTPServerCapsHeaderSize drives a real connection: an oversized header
// must be refused by the server rather than buffered.
func TestNewHTTPServerCapsHeaderSize(t *testing.T) {
	router := gin.New()
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	srv := newHTTPServer(":0", router)
	srv.MaxHeaderBytes = 4096

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve(ln)
	defer srv.Close()

	req, err := http.NewRequest("GET", "http://"+ln.Addr().String()+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Huge", strings.Repeat("z", 16384))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// A refused oversized header may also surface as a closed connection.
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("oversized header answered %d, want %d or a dropped connection",
			resp.StatusCode, http.StatusRequestHeaderFieldsTooLarge)
	}
}

// TestNewHTTPServerCarriesTheDefaultLimits pins the values the production
// server is built with, so a future edit cannot quietly drop them.
func TestNewHTTPServerCarriesTheDefaultLimits(t *testing.T) {
	srv := newHTTPServer(":22880", gin.New())

	if srv.MaxHeaderBytes != maxHeaderBytes {
		t.Fatalf("MaxHeaderBytes = %d, want %d", srv.MaxHeaderBytes, maxHeaderBytes)
	}
	if srv.Addr != ":22880" {
		t.Fatalf("Addr = %q, want %q", srv.Addr, ":22880")
	}
	if srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 {
		t.Fatalf("timeouts lost: %v", []time.Duration{srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout})
	}
}

func TestW11_ExemptWriteTimeoutLetsASlowHandlerAnswer(t *testing.T) {
	slow := func(c *gin.Context) {
		time.Sleep(600 * time.Millisecond)
		c.String(http.StatusOK, "ok")
	}
	router := gin.New()
	router.GET("/exempt", exemptWriteTimeout(), slow)
	router.GET("/plain", slow)
	srv := httptest.NewUnstartedServer(router)
	srv.Config.WriteTimeout = 200 * time.Millisecond
	srv.Start()
	defer srv.Close()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	resp, err := client.Get(srv.URL + "/exempt")
	if err != nil {
		t.Fatalf("/exempt: %v, want an answer", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ok" {
		t.Errorf("/exempt answered %q, want ok", body)
	}

	resp, err = client.Get(srv.URL + "/plain")
	if err == nil {
		resp.Body.Close()
		t.Fatal("/plain answered although the write deadline passed while the handler slept")
	}
	if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.ECONNRESET) {
		t.Errorf("/plain failed with %v, want a closed connection", err)
	}
}
