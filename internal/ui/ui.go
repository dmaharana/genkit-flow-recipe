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
	return http.FileServer(http.FS(distFS))
}
