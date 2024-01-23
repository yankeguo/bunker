package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
)

type DataDir string

func (d DataDir) String() string {
	return string(d)
}

func main() {
	var optDataDir string

	flag.StringVar(&optDataDir, "data-dir", "", "data directory")
	flag.Parse()

	app := fx.New(
		fx.Supply(DataDir(optDataDir)),
		fx.Provide(createDatabase),
		fx.Provide(createSSHServerParams, createSSHServer, createSigners),
		ufx.ProvideConfFromYAMLFile(filepath.Join(optDataDir, "bunker.yaml")),
		ufx.Module,
		fx.Invoke(installStatic, installAPIAuthorizedKeys),
		fx.Invoke(func(s *SSHServer) {}),
		fx.Invoke(initializeAdminUsers),
	)
	if app.Err() != nil {
		log.Println(app.Err().Error())
		os.Exit(1)
	}
	app.Run()
}
