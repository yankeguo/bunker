package bunker

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"

	"github.com/yankeguo/rg"
	"github.com/yankeguo/ufx"
)

//go:embed ui/.output/public ui/.output/public/**/*
var STATIC embed.FS

func InstallStaticToRouter(ur ufx.Router) {
	if Debug("ui") {
		proxy := httputil.NewSingleHostReverseProxy(rg.Must(url.Parse("http://localhost:3000")))
		ur.ServeMux().Handle("/", proxy)
	} else {
		f := rg.Must(fs.Sub(STATIC, path.Join("ui", ".output", "public")))
		ur.ServeMux().Handle("/", http.FileServer(http.FS(f)))
	}
}
