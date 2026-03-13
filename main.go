package main

import (
	"embed"
	"log"
	"os"

	"video-list/internal/backend"
)

//go:embed dist/*
var embeddedFiles embed.FS

func main() {
	if err := backend.Run(embeddedFiles, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
