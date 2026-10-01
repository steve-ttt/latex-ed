package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// GetAssetsFS returns the sub-filesystem of embedded dist assets.
func GetAssetsFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
