package web

import (
	"embed"
	"io/fs"
)

// Assets contains the frontend build copied by the release scripts.
// The tracked index.html is a safe development fallback; release builds replace
// this directory with the output of `npm run build` before compiling Go.
//
//go:embed dist/*
var embedded embed.FS

func Assets() fs.FS {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return assets
}
