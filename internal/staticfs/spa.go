// Package staticfs serves the embedded React SPA (or a STATIC_DIR override) with
// history-API fallback and the SPA cache-header policy (behavioral-map §7, D9/D17).
package staticfs

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// embeddedRoot is the embedded build with the "dist/" prefix stripped.
var embeddedRoot = mustSub(distFS, "dist")

func mustSub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err) // dist is embedded at build time, so this cannot fail
	}
	return sub
}

// Handler returns an http.Handler serving the SPA, or nil when SPA serving is disabled
// (STATIC_DISABLED — the no_spa profile). A STATIC_DIR pointing at an existing directory
// overrides the embedded build (dev / the contract with_spa profile); otherwise the
// embedded dist is served.
func Handler(staticDir string, disabled bool) http.Handler {
	if disabled {
		return nil
	}
	root := embeddedRoot
	if isDir(staticDir) {
		root = os.DirFS(staticDir)
	}
	return &spaHandler{root: root}
}

func isDir(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

type spaHandler struct {
	root fs.FS
}

// ServeHTTP serves a real file with an immutable cache, else falls back to index.html with
// no-cache (history-API routing). It is only reached for non-/api paths — the /api
// subrouter owns those and returns the JSON 404 for unknown ones.
func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name != "" && name != "index.html" && !strings.HasSuffix(name, "/") {
		if f, info, ok := openFile(h.root, name); ok {
			defer func() { _ = f.Close() }()
			w.Header().Set("Cache-Control", "public, immutable, max-age=31536000")
			http.ServeContent(w, r, name, info.ModTime(), f)
			return
		}
	}
	h.serveIndex(w, r)
}

func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	f, info, ok := openFile(h.root, "index.html")
	if !ok {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", info.ModTime(), f)
}

// openFile opens a regular file from root as a ReadSeeker for http.ServeContent.
func openFile(root fs.FS, name string) (io.ReadSeekCloser, fs.FileInfo, bool) {
	f, err := root.Open(name)
	if err != nil {
		return nil, nil, false
	}
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		_ = f.Close()
		return nil, nil, false
	}
	rs, ok := f.(io.ReadSeekCloser)
	if !ok {
		_ = f.Close()
		return nil, nil, false
	}
	return rs, info, true
}
