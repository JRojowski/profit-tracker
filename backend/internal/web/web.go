// Package web holds the built frontend, embedded in the binary.
//
// The frontend build copies its output into the dist folder.
// Without a frontend build, dist holds only .gitkeep, and the server tells the user to build the frontend.
package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the files of the built frontend, with index.html at the root.
func Dist() (fs.FS, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	return sub, nil
}
