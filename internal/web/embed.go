package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var embeddedFiles embed.FS

func GetFS() (fs.FS, error) {
	sub, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		return nil, err
	}
	return sub, nil
}
