package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yankeguo/bunker"
)

const (
	startupDelay  = 3 * time.Second
	shutdownDelay = 3 * time.Second
)

func main() {
	os.Exit(run())
}

func run() int {
	var dataDir string
	flag.StringVar(&dataDir, "data-dir", "", "data directory")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dir := bunker.DataDir(dataDir)
	cfg, err := bunker.LoadConfig(dir)
	if err != nil {
		log.Error("load config", "err", err)
		return 1
	}

	db, err := bunker.CreateDatabase(dir)
	if err != nil {
		log.Error("open database", "err", err)
		return 1
	}
	defer bunker.CloseDatabase(db)

	signers, err := bunker.CreateSigners(log, dir)
	if err != nil {
		log.Error("load ssh keys", "err", err)
		return 1
	}
	if err = bunker.InitializeUsers(log, dir, db); err != nil {
		log.Error("initialize users", "err", err)
		return 1
	}

	app := bunker.CreateApp(db, cfg, log, signers)
	sshServer := bunker.NewSSHServer(cfg, db, signers, log)
	httpServer := bunker.NewHTTPServer(cfg, app, log)

	errCh := make(chan error, 2)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		if err := sshServer.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		log.Error("server exited", "err", err)
		shutdown(log, httpServer, sshServer, false)
		return 1
	case <-time.After(startupDelay):
	case <-ctx.Done():
	}

	code := 0
	select {
	case err := <-errCh:
		log.Error("server exited", "err", err)
		code = 1
	case <-ctx.Done():
		log.Info("shutting down")
	}
	shutdown(log, httpServer, sshServer, code == 0)
	return code
}

func shutdown(log *slog.Logger, httpServer *bunker.HTTPServer, sshServer *bunker.SSHServer, drain bool) {
	if drain {
		time.Sleep(shutdownDelay)
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownDelay)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("http shutdown", "err", err)
	}
	if err := sshServer.Shutdown(ctx); err != nil {
		log.Error("ssh shutdown", "err", err)
	}
}
