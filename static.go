package bunker

import (
	"embed"
	"io/fs"
	"net/http"
	"path"

	"github.com/yankeguo/rg"
	"github.com/yankeguo/ufx"
)

//go:embed ui/.output/public ui/.output/public/**/*
var STATIC embed.FS

func InstallStaticToRouter(ur ufx.Router) {
	f := rg.Must(fs.Sub(STATIC, path.Join("ui", ".output", "public")))
	ur.ServeMux().Handle("/", http.FileServer(http.FS(f)))
}
