package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/yankeguo/rg"
	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
)

func installStatic(ur ufx.Router) {
	f := rg.Must(fs.Sub(STATIC, path.Join("ui", ".output", "public")))
	ur.ServeMux().Handle("/", http.FileServer(http.FS(f)))
}

func main() {
	app := fx.New(
		ufx.ProvideConfFromYAMLFile("conf.yaml"),
		ufx.Module,
		fx.Invoke(installStatic),
	)
	if app.Err() != nil {
		log.Println(app.Err().Error())
		os.Exit(1)
	}
	app.Run()
}
