package server

import (
	"io/fs"
	"net/http"
	"strings"
)

// NewSPAHandler creates an http.Handler that serves static files from webFS,
// falling back to index.html for Single-Page Application routing.
func NewSPAHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if file exists in the filesystem
		if f, err := webFS.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// If file doesn't exist, serve index.html (SPA client-side routing)
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
