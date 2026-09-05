// Package web embeds the built SPA (web/dist, produced by `npm run
// build`) so it ships inside the compiled Go binary. Run the frontend
// build before `go build` — the embed directive requires dist/ to exist.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// DistFS is the built SPA's static file tree, rooted at dist/ (i.e. it
// contains "index.html" and "assets/...", not "dist/index.html").
var DistFS fs.FS = mustSub(distFS, "dist")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
