package web

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

// notBuiltPage is what a binary without a frontend build answers on every
// route of the single-page app. dist/ then holds only .gitkeep, and a blank
// page would give no hint what is wrong.
const notBuiltPage = `<!doctype html><title>lif-serve</title><h1>Web UI not built</h1><p>This lif-serve binary was built without the frontend. Run <code>just build</code> in the lif repository and start the new binary.</p>`

// DistFS returns the embedded frontend filesystem, rooted at dist/.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// Built reports whether this binary embeds a frontend build.
func Built() bool {
	fsys, err := DistFS()
	if err != nil {
		return false
	}
	return hasIndex(fsys)
}

func hasIndex(fsys fs.FS) bool {
	_, err := fs.Stat(fsys, "index.html")
	return err == nil
}

// Handler returns an http.Handler that serves the embedded SPA, see handlerFor.
func Handler() http.Handler {
	fsys, err := DistFS()
	if err != nil {
		// This shouldn't happen with embed, but handle gracefully
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not available", http.StatusInternalServerError)
		})
	}
	return handlerFor(fsys)
}

// maxRouteDepth is the number of slashes below which no route of the app
// lies (/workspace/<id> has one). Deeper paths are not routes, and answering
// them would let a short request ask for a long page (see rewriteIndex).
const maxRouteDepth = 8

// handlerFor serves the single-page app of fsys: a file of the build under its
// own path, a page the build prerendered (planning.html answers /planning),
// and index.html for every other route, which the client router resolves.
//
// A proxy may mount the UI under a prefix and strip it before the request
// reaches this server (tailscale serve --set-path, nginx proxy_pass with a
// trailing slash, Caddy handle_path). The server cannot know that prefix, so
// no page may name an asset or the app root by an absolute path. The pages the
// build prerendered are relative already; index.html is rewritten per request
// to point at the app root from the URL it answers (see rewriteIndex).
func handlerFor(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	index, err := fs.ReadFile(fsys, "index.html")
	built := err == nil

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")

		// Without a frontend build every route of the app answers with the
		// notice; only paths that name a file (missing assets) stay 404.
		if !built && !strings.Contains(p, ".") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, notBuiltPage)
			return
		}

		// A file of the build. A directory is no route and is never listed.
		if info, err := fs.Stat(fsys, p); err == nil && info.Mode().IsRegular() {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Actual missing file (e.g., missing .js, .css)
		if strings.Contains(p, ".") {
			http.NotFound(w, r)
			return
		}

		// A page the build prerendered. Not for a path with a trailing slash:
		// its relative links are written for the URL without one.
		if p != "" && !strings.HasSuffix(p, "/") {
			if page, err := fs.ReadFile(fsys, p+".html"); err == nil {
				serveHTML(w, page)
				return
			}
		}

		// Any other route of the app. The depth is that of the URL the browser
		// holds, so a %2F in a segment is no level.
		depth := strings.Count(strings.TrimPrefix(r.URL.EscapedPath(), "/"), "/")
		if depth > maxRouteDepth {
			http.NotFound(w, r)
			return
		}
		serveHTML(w, rewriteIndex(index, depth))
	})
}

// serveHTML answers 200 explicitly: behind gin's NoRoute the status is 404
// until a handler says otherwise.
func serveHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The page names assets by hashed file names, which change with a new
	// build: ask the server every time.
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// rewriteIndex turns the fallback page of the build, which names the app root
// by absolute paths, into one for a URL that has depth slashes below the app
// root: assets are "./_app/…" at the root and "../_app/…" one level down. The
// client needs the root as a path too, SvelteKit's base (empty in the build).
// It gets it from the page location, as the prerendered pages do.
func rewriteIndex(index []byte, depth int) []byte {
	root := "."
	if depth > 0 {
		root = strings.TrimSuffix(strings.Repeat("../", depth), "/")
	}
	index = bytes.ReplaceAll(index, []byte(`"/_app/`), []byte(`"`+root+`/_app/`))
	return bytes.ReplaceAll(index, []byte(`base: ""`), []byte(`base: new URL("`+root+`", location).pathname.slice(0, -1)`))
}
