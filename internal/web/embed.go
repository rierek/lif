package web

import (
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

// Handler returns an http.Handler that serves the embedded SPA.
// It handles SPA routing by serving index.html for paths that don't match a file.
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

func handlerFor(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	built := hasIndex(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Without a frontend build every route of the app answers with the
		// notice; only paths that name a file (missing assets) stay 404.
		if !built && !strings.Contains(path, ".") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, notBuiltPage)
			return
		}

		// Clean the path
		if path == "/" {
			path = "/index.html"
		}

		// Try to open the file
		path = strings.TrimPrefix(path, "/")
		f, err := fsys.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found - serve index.html for SPA routing
		// But only for paths that look like routes (not missing assets)
		if !strings.Contains(path, ".") {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Actual missing file (e.g., missing .js, .css)
		http.NotFound(w, r)
	})
}
