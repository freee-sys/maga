package http

import (
	"io/fs"
	"net/http"
)

// NewSPAHandler serves a built single-page app from fsys: static files
// as-is, and index.html for any other path so client-side routing
// (React Router) keeps working on a full page load or refresh.
func NewSPAHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(fsys, path); err != nil {
			// http.FileServer has a special case that 301-redirects any
			// request literally ending in "/index.html" back to "/" —
			// request "/" instead, which serves the same file without
			// the redirect.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
