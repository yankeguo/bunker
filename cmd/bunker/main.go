package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/yankeguo/bunker"
	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

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
		fx.Supply(
			bunker.DataDir(optDataDir),
			logger,
			logger.Sugar(),
		),

		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),

		ufx.ProvideConfFromYAMLFile(filepath.Join(optDataDir, "config.yaml")),
		ufx.Module,

		fx.Provide(
			bunker.CreateDatabase,
			bunker.CreateSSHServer,
			bunker.CreateSigners,
			bunker.CreateApp,
		),

		fx.Invoke(
			bunker.InitializeUsers,
			bunker.InstallStaticToRouter,
			bunker.InstallSignersToRouter,
			bunker.InstallAppToRouter,
		),

		fx.Invoke(func(s *bunker.SSHServer) {}),
	)
	if app.Err() != nil {
		log.Println(app.Err().Error())
		os.Exit(1)
	}
	app.Run()
}
