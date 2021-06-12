// +build !noViewer go1.16

package ui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-spatial/tegola/internal/log"
)

//Embed UI dist Folder recursively
//go:embed dist/*
var dist embed.FS

func GetDistFileSystem() http.FileSystem {
	distfs, err := fs.Sub(dist, "dist")
	if err != nil {
		log.Fatal(err)
	}
	return http.FS(distfs)
}
