package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var static embed.FS

// Handler serves the mobile browser upload page and static assets.
func Handler() http.Handler {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
