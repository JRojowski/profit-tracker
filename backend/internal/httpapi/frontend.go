package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// frontendNotBuilt is the page text when the binary has no built frontend.
const frontendNotBuilt = "profit-tracker: the frontend is not built into this binary.\n" +
	"Build the frontend, then build the Go binary again.\n" +
	"The API is available under /api.\n"

// frontendHandler serves the files of the built frontend.
// For a client route with no file, it serves index.html, so that the React router can show the page.
// A missing path with a file extension, or under /assets/, is a missing file. It gets 404, not index.html.
func frontendHandler(frontend fs.FS) http.Handler {
	if !hasFile(frontend, "index.html") {
		return http.HandlerFunc(frontendNotBuiltHandler)
	}

	files := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowRead(w, r) {
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/")
		switch {
		case name == "" || hasFile(frontend, name):
			files.ServeHTTP(w, r)
		case isFilePath(name):
			http.NotFound(w, r)
		default:
			http.ServeFileFS(w, r, frontend, "index.html")
		}
	})
}

// frontendNotBuiltHandler tells the user to build the frontend.
func frontendNotBuiltHandler(w http.ResponseWriter, r *http.Request) {
	if !allowRead(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(frontendNotBuilt))
}

// allowRead answers 405 for a method other than GET and HEAD, and returns false.
func allowRead(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return false
}

// isFilePath tells if name is the path of a file, not a client route:
// it is under assets/, or its last element has an extension.
func isFilePath(name string) bool {
	return strings.HasPrefix(name, "assets/") || path.Ext(name) != ""
}

// hasFile tells if name is a regular file in fsys.
func hasFile(fsys fs.FS, name string) bool {
	if fsys == nil || !fs.ValidPath(name) {
		return false
	}
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
