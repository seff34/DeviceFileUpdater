package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist holds the Vite build (make ui). .keep keeps the directory embeddable
// before the first build.
//
//go:embed all:dist
var distFS embed.FS

const notBuilt = `<!doctype html><meta charset="utf-8"><title>DeviceFileUpdater</title>
<p>Arayüz derlenmemiş. Kaynaktan çalıştırıyorsanız önce <code>make ui</code> çalıştırın.</p>`

func staticHandler() http.Handler {
	sub, _ := fs.Sub(distFS, "dist")
	files := http.FileServerFS(sub)
	index, indexErr := fs.ReadFile(sub, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(sub, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		// SPA fallback: every unknown non-API path renders the app.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		if indexErr != nil {
			w.Write([]byte(notBuilt))
			return
		}
		w.Write(index)
	})
}
