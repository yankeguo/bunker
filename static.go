package bunker

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/yankeguo/rg"
	"github.com/yankeguo/ufx"
)

//go:embed ui/.output/public ui/.output/public/**/*
var STATIC embed.FS

// spaHandler serves embedded static files and falls back to the generated
// 404.html for unknown paths, instead of a plain text 404
type spaHandler struct {
	fsys fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fileServer := http.FileServer(http.FS(h.fsys))

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

	if name == "" {
		fileServer.ServeHTTP(w, r)
		return
	}

	if info, err := fs.Stat(h.fsys, name); err == nil {
		if !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(h.fsys, path.Join(name, "index.html")); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
	}

	if buf, err := fs.ReadFile(h.fsys, "404.html"); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(buf)
		return
	}

	http.NotFound(w, r)
}

func InstallStaticToRouter(ur ufx.Router) {
	if Debug("ui") {
		proxy := httputil.NewSingleHostReverseProxy(rg.Must(url.Parse("http://localhost:3000")))
		ur.ServeMux().Handle("/", proxy)
	} else {
		f := rg.Must(fs.Sub(STATIC, path.Join("ui", ".output", "public")))
		ur.ServeMux().Handle("/", spaHandler{fsys: f})
	}
}
