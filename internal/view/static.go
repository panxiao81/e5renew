package view

import (
	"embed"
	"net/http"
)

//go:embed statics/*
var staticFS embed.FS

func StaticFileHandler() http.Handler {
	return http.FileServerFS(staticFS)
}
