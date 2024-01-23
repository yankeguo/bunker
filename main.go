package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

type DataDir string

func (d DataDir) String() string {
	return string(d)
}

func main() {
	var optDataDir string

	flag.StringVar(&optDataDir, "data-dir", "", "data directory")
	flag.Parse()

	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	logger, err := loggerConfig.Build()

	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	defer logger.Sync()

	app := fx.New(
		fx.Supply(logger),
		fx.Supply(logger.Sugar()),
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		fx.Supply(DataDir(optDataDir)),
		fx.Provide(createDatabase),
		fx.Provide(createSSHServerParams, createSSHServer, createSigners),
		ufx.ProvideConfFromYAMLFile(filepath.Join(optDataDir, "bunker.yaml")),
		ufx.Module,
		fx.Invoke(installStatic, installAPIAuthorizedKeys),
		fx.Invoke(func(s *SSHServer) {}),
		fx.Invoke(initializeUsers),
	)
	if app.Err() != nil {
		log.Println(app.Err().Error())
		os.Exit(1)
	}
	app.Run()
}
