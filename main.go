package main

import (
	"embed"

	"video-list/internal/app"
)

//go:embed dist/*
var embeddedFiles embed.FS

func main() {
	app.Start(embeddedFiles)
}
