package routes

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// RegisterStaticSPA serves bundled frontend assets and falls back to index.html.
func RegisterStaticSPA(mux *http.ServeMux, distFS fs.FS) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		path = strings.TrimPrefix(path, "/")

		if content, err := fs.ReadFile(distFS, path); err == nil {
			ext := filepath.Ext(path)
			contentType := mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(content)
			return
		}

		content, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(content)
	})
}
