package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHandlerWithoutFrontendBuild(t *testing.T) {
	h := handlerFor(fstest.MapFS{})

	for _, path := range []string{"/", "/workspace/x"} {
		t.Run("serves the notice for "+path, func(t *testing.T) {
			rec := get(h, path)
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
			}
			if body := rec.Body.String(); !strings.Contains(body, "<h1>Web UI not built</h1>") || !strings.Contains(body, "just build") {
				t.Errorf("body does not carry the notice: %q", body)
			}
		})
	}

	t.Run("keeps 404 for a missing asset", func(t *testing.T) {
		if rec := get(h, "/x.js"); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestHandlerWithFrontendBuild(t *testing.T) {
	h := handlerFor(fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}})

	rec := get(h, "/")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "app") {
		t.Errorf("body is not index.html: %q", rec.Body.String())
	}
}

func TestDistFS(t *testing.T) {
	t.Run("returns filesystem rooted at dist", func(t *testing.T) {
		fsys, err := DistFS()
		if err != nil {
			t.Fatalf("DistFS() error = %v", err)
		}
		if fsys == nil {
			t.Error("DistFS() returned nil filesystem")
		}
	})
}
