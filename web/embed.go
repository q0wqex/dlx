package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html style.css app.js
var webFS embed.FS

// Handler returns an http.Handler serving the embedded frontend assets.
func Handler() http.Handler {
	sub, err := fs.Sub(webFS, ".")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
