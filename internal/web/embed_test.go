package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
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

// build is the shape of a SvelteKit static build: the fallback page names the
// app root by absolute paths and has an empty base, the prerendered pages are
// relative already.
func build() fstest.MapFS {
	return fstest.MapFS{
		"index.html":          {Data: []byte(`<link href="/_app/entry.js" rel="modulepreload"><script>{ __sveltekit_x = { base: "", version: "1" }; import("/_app/entry.js"); }</script>`)},
		"planning.html":       {Data: []byte(`<link href="./_app/entry.js"><script>{ base: new URL(".", location).pathname.slice(0, -1) }</script>`)},
		"planning/board.html": {Data: []byte(`<link href="../_app/entry.js"><script>{ base: new URL("..", location).pathname.slice(0, -1) }</script>`)},
		"_app/entry.js":       {Data: []byte("export {}")},
	}
}

// A proxy may strip a prefix, so the page may not name the app root by an
// absolute path: it names it relative to the URL it answers.
func TestHandlerPointsTheFallbackPageAtTheAppRoot(t *testing.T) {
	h := handlerFor(build())

	cases := []struct{ path, prefix, root string }{
		{"/", "./", "."},
		{"/workspace/abc", "../", ".."},
		{"/workspace/abc/", "../../", "../.."},
		{"/workspace/a%2Fb", "../", ".."}, // the browser sees one level
	}
	for _, tt := range cases {
		t.Run(tt.path, func(t *testing.T) {
			rec := get(h, tt.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", cc)
			}
			body := rec.Body.String()
			for _, want := range []string{
				`href="` + tt.prefix + `_app/entry.js"`,
				`import("` + tt.prefix + `_app/entry.js")`,
				`base: new URL("` + tt.root + `", location).pathname.slice(0, -1)`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("body lacks %q: %s", want, body)
				}
			}
			for _, ban := range []string{`"/_app/`, `base: ""`} {
				if strings.Contains(body, ban) {
					t.Errorf("body still has %q: %s", ban, body)
				}
			}
		})
	}
}

func TestHandlerServesWhatTheBuildHolds(t *testing.T) {
	fsys := build()
	h := handlerFor(fsys)

	t.Run("a prerendered page answers its route as it is", func(t *testing.T) {
		for path, file := range map[string]string{"/planning": "planning.html", "/planning/board": "planning/board.html"} {
			rec := get(h, path)
			if rec.Code != http.StatusOK || rec.Body.String() != string(fsys[file].Data) {
				t.Errorf("%s: status %d, body %q; want the content of %s", path, rec.Code, rec.Body.String(), file)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
				t.Errorf("%s: Content-Type = %q, want text/html; charset=utf-8", path, ct)
			}
		}
	})

	t.Run("a file is served as it is", func(t *testing.T) {
		if rec := get(h, "/_app/entry.js"); rec.Code != http.StatusOK || rec.Body.String() != "export {}" {
			t.Errorf("status %d, body %q; want the file", rec.Code, rec.Body.String())
		}
	})

	t.Run("a directory is a route of the app, not a listing", func(t *testing.T) {
		rec := get(h, "/_app")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "__sveltekit_x") || strings.Contains(rec.Body.String(), "<a href") {
			t.Errorf("status %d, body %q; want the fallback page", rec.Code, rec.Body.String())
		}
	})

	t.Run("a missing file is 404", func(t *testing.T) {
		if rec := get(h, "/missing.js"); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("a path deeper than any route is 404", func(t *testing.T) {
		if rec := get(h, "/"+strings.Repeat("a/", maxRouteDepth+1)+"b"); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
		if rec := get(h, "/"+strings.Repeat("a/", maxRouteDepth)+"b"); rec.Code != http.StatusOK {
			t.Errorf("status = %d at the limit, want 200", rec.Code)
		}
	})
}

// serve.go mounts the handler as gin's NoRoute, where a response that sets no
// status is a 404.
func TestHandlerAnswers200BehindGinNoRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.NoRoute(gin.WrapH(handlerFor(build())))

	for _, path := range []string{"/", "/planning", "/planning/board", "/workspace/abc", "/_app/entry.js"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
		}
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
