// Package webui serves the embedded console: one shell for the dashboard pages
// and the chat workspace. Assets live in this directory so the binary stays a
// single file with no CDN or runtime asset path.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed index.html assets/*
var files embed.FS

// Page serves the single-page shell. /dashboard and /chat both use it; the
// client picks the initial page from the URL or the hash.
func Page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	body, err := files.ReadFile("index.html")
	if err != nil {
		http.Error(w, "webui unavailable", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

// Asset serves files under /assets/. The embedded names are hashed by content
// only at release time, so tell browsers to revalidate instead of caching
// forever; the files are small.
func Asset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	if name == "" || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	sub, err := fs.Sub(files, "assets")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	req := r.Clone(r.Context())
	req.URL.Path = "/" + name
	http.FileServer(http.FS(sub)).ServeHTTP(w, req)
}
