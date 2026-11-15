package web

import (
	"embed"
	"io/fs"
)

//go:embed html css js
var files embed.FS

func Files() fs.FS {
	return files
}
