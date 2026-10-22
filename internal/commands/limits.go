package commands

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// maxHeaderBytes caps the request header a connection may send before the
	// server drops it.
	maxHeaderBytes = 1 << 20 // 1 MB

	// maxRequestBodyBytes caps a single request body. GraphQL documents and
	// item bodies are far below this; agent image attachments are the largest
	// legitimate payload.
	maxRequestBodyBytes = 10 << 20 // 10 MB

	// maxWebSocketConnections caps concurrent upgraded connections across all
	// WebSocket endpoints. Each one holds a file descriptor and a goroutine for
	// its whole lifetime, so an unbounded count is a way to exhaust both.
	maxWebSocketConnections = 100
)

// maxBodyMiddleware bounds how much of a request body a handler can read.
// http.MaxBytesReader fails the read rather than buffering, so an oversized
// upload never becomes an allocation.
func maxBodyMiddleware(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

// connLimiter caps the number of concurrently open WebSocket connections. One
// limiter is shared by every upgrade endpoint, because the resource they
// compete for — this process's file descriptors — is shared too.
type connLimiter struct {
	max  int
	mu   sync.Mutex
	open int
}

func newConnLimiter(max int) *connLimiter {
	return &connLimiter{max: max}
}

// acquire takes a slot, reporting whether one was free.
func (l *connLimiter) acquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.open >= l.max {
		return false
	}
	l.open++
	return true
}

func (l *connLimiter) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.open > 0 {
		l.open--
	}
}

// Middleware rejects an upgrade request once the cap is reached, before the
// handshake completes. Plain requests pass through uncounted: they return in
// milliseconds, while an upgraded connection lives until the client leaves.
//
// The slot is held for the length of the handler because both WebSocket
// handlers block for the lifetime of the connection — gqlgen's transport runs
// its read loop inside ServeHTTP, and handleTerminalWS does the same.
func (l *connLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isWebSocketUpgrade(c.Request) {
			c.Next()
			return
		}
		if !l.acquire() {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		defer l.release()
		c.Next()
	}
}

// isWebSocketUpgrade reports whether the request asks for a WebSocket upgrade.
func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// newHTTPServer builds the server with the timeouts and header limit the
// service runs with.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: maxHeaderBytes,
	}
}

// exemptWriteTimeout clears the write deadline of one request. net/http arms
// WriteTimeout once it has read the request headers, so a handler that runs
// longer than the timeout writes its response to a closed connection. A
// mutation that runs a hook or a gate is such a handler: the write is already
// committed when the client sees a failure, and a repeat would duplicate a
// createItem. Only the request that carries this handler is exempt; every
// other route keeps the WriteTimeout of newHTTPServer. gin's writer unwraps to
// the net/http writer, so the controller reaches the connection.
func exemptWriteTimeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
		c.Next()
	}
}
