package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler serves the embedded Vue SPA. API and WebSocket paths return 404 so
// they are never matched by the file server. Unknown paths fall back to
// index.html so the Vue router can handle client-side navigation.
func SPAHandler(frontendFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(frontendFS))

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/ws") {
			http.NotFound(w, r)
			return
		}

		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		if _, err := fs.Stat(frontendFS, path); err != nil {
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	}
}
