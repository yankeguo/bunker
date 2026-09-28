package main

import (
	"flag"
	"log"
	"os"

	"github.com/yankeguo/bunker"
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

		fx.Provide(
			bunker.LoadConfig,
			bunker.CreateDatabase,
			bunker.CreateSSHServer,
			bunker.CreateSigners,
			bunker.CreateApp,
			bunker.NewHTTPServer,
		),

		fx.Invoke(
			bunker.InitializeUsers,
			func(*bunker.HTTPServer) {},
			func(*bunker.SSHServer) {},
		),
	)
	if app.Err() != nil {
		log.Println(app.Err().Error())
		os.Exit(1)
	}
	app.Run()
}
