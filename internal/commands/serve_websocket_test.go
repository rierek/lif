package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"

	"github.com/rierek/lif/internal/cors"
	"github.com/rierek/lif/internal/graph"
)

// TestGqlWebsocketImplementationRejectsDisallowedOrigin pins the origin check
// that replaced gorilla/websocket's Upgrader.CheckOrigin when gqlgen moved to
// the pluggable WebsocketImplementation interface (v0.17.9x). A regression
// here would let any origin open a GraphQL websocket (CSWSH).
func TestGqlWebsocketImplementationRejectsDisallowedOrigin(t *testing.T) {
	checker := cors.NewChecker([]string{"http://allowed.example"})
	impl := gqlWebsocketImplementation{checkOrigin: checker.CheckOriginFunc()}

	req := httptest.NewRequest(http.MethodGet, "/api/graphql", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()

	conn, err := impl.Accept(rec, req, transport.WebsocketAcceptOptions{})
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected Accept to reject a disallowed origin, got nil error")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a disallowed origin, got %d", rec.Code)
	}
}

// TestGqlWebsocketImplementationAcceptsAllowedOrigin proves the allowed path
// still completes a real websocket handshake through coder/websocket, the
// implementation gqlgen now defaults to.
func TestGqlWebsocketImplementationAcceptsAllowedOrigin(t *testing.T) {
	checker := cors.NewChecker([]string{"http://allowed.example"})
	impl := gqlWebsocketImplementation{checkOrigin: checker.CheckOriginFunc()}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := impl.Accept(w, r, transport.WebsocketAcceptOptions{})
		if err != nil {
			return
		}
		defer conn.Close()
		conn.NextReader() //nolint:errcheck // block until the client closes
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + srv.URL[len("http"):]
	c, _, err := coderws.Dial(ctx, url, &coderws.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://allowed.example"}},
	})
	if err != nil {
		t.Fatalf("expected the allowed origin to complete the handshake, got: %v", err)
	}
	c.Close(coderws.StatusNormalClosure, "")
}

// TestGraphQLWebsocketHandshakeThroughGinRouter pins the full transport stack
// exactly as runServer wires it: gqlHandler mounted via gin.WrapH on a real
// *gin.Engine, not a bare http.HandlerFunc. This is the layer that actually
// broke: coder/websocket's Accept calls w.WriteHeader(101) (with a
// Gin-specific WriteHeaderNow() flush) *before* hj.Hijack(), and Gin
// >=1.11.0 rejects any Hijack() call after a WriteHeader() with "gin:
// response already written" (github.com/coder/websocket#538). The HTTP
// upgrade response (including the negotiated subprotocol) still reaches the
// client, so coderws.Dial succeeds either way -- only actually completing
// the graphql-transport-ws connection_init/connection_ack roundtrip proves
// the hijacked connection is alive. Fixed by gin v1.12.0
// (gin-gonic/gin#4373); a regression here means every GraphQL subscription
// (agent chat, item list, worktree status, ...) silently never delivers data.
func TestGraphQLWebsocketHandshakeThroughGinRouter(t *testing.T) {
	checker := cors.NewChecker([]string{"http://allowed.example"})

	es := graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}})
	gqlHandler := handler.New(es)
	gqlHandler.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Implementation:        gqlWebsocketImplementation{checkOrigin: checker.CheckOriginFunc()},
	})
	gqlHandler.AddTransport(transport.Options{})
	gqlHandler.AddTransport(transport.GET{})
	gqlHandler.AddTransport(transport.POST{})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Any("/api/graphql", gin.WrapH(gqlHandler))

	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + srv.URL[len("http"):] + "/api/graphql"
	c, _, err := coderws.Dial(ctx, url, &coderws.DialOptions{
		Subprotocols: []string{"graphql-transport-ws"},
		HTTPHeader:   http.Header{"Origin": []string{"http://allowed.example"}},
	})
	if err != nil {
		t.Fatalf("dial through the real gin router failed: %v", err)
	}
	defer c.Close(coderws.StatusNormalClosure, "")

	if err := wsjson.Write(ctx, c, map[string]any{"type": "connection_init"}); err != nil {
		t.Fatalf("failed to send connection_init: %v", err)
	}

	var ack map[string]any
	if err := wsjson.Read(ctx, c, &ack); err != nil {
		t.Fatalf("never received connection_ack through the real gin router -- the server-side "+
			"hijack likely failed silently after the HTTP upgrade response was already sent: %v", err)
	}
	if ack["type"] != "connection_ack" {
		t.Fatalf("expected connection_ack, got %v", ack)
	}
}
