package remote

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The phone app (decision 0021) is built into the binary (frontend/dist/m)
// and served at /m/. Its page gets a content security policy: its own
// scripts only (and WebAssembly, for the code highlighter), no plugins,
// connections only to this server, so markup that slipped into an answer
// couldn't send the device's token anywhere. The page and its service
// worker are always checked for a newer copy; built assets, whose names
// change with their content, are kept.

const pwaCSP = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'self' blob: data:; " +
	"worker-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

func servePWA(files fs.FS) http.Handler {
	fileServer := http.StripPrefix("/m/", http.FileServerFS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		name := strings.TrimPrefix(r.URL.Path, "/m/")
		// No folder listings: a folder other than the app's own is not found.
		if name != "" && strings.HasSuffix(name, "/") {
			http.NotFound(w, r)
			return
		}
		switch {
		case name == "" || name == "index.html":
			h.Set("Cache-Control", "no-cache")
			h.Set("Content-Security-Policy", pwaCSP)
		case name == "sw.js":
			h.Set("Cache-Control", "no-cache")
			h.Set("Content-Type", "text/javascript; charset=utf-8")
		case path.Ext(name) == ".webmanifest":
			h.Set("Cache-Control", "no-cache")
			h.Set("Content-Type", "application/manifest+json")
		case strings.HasPrefix(name, "assets/"):
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}
