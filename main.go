package main

import (
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/yankeguo/rg"
	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
)

func installStatic(ur ufx.Router) {
	f := rg.Must(fs.Sub(STATIC, path.Join("ui", ".output", "public")))
	ur.ServeMux().Handle("/", http.FileServer(http.FS(f)))
}

func main() {
	var optDataDir string

	flag.StringVar(&optDataDir, "data-dir", "", "data directory")
	flag.Parse()

	app := fx.New(
		ufx.ProvideConfFromYAMLFile(filepath.Join(optDataDir, "bunker.yaml")),
		ufx.Module,
		fx.Invoke(installStatic),
	)
	if app.Err() != nil {
		log.Println(app.Err().Error())
		os.Exit(1)
	}
	app.Run()
}
