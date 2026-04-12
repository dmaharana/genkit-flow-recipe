package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var uiDist embed.FS

// Handler returns an http.Handler that serves the UI static files.
func Handler() http.Handler {
	// Root of the UI build
	distFS, err := fs.Sub(uiDist, "dist")
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		f, err := distFS.Open(r.URL.Path[1:])
		if err == nil {
			f.Close()
			http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
			return
		}
		// If not found, serve index.html
		data, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
}
