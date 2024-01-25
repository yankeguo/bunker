package bunker

import "github.com/yankeguo/ufx"

type App struct{}

func CreateApp() (app *App, err error) {
	app = &App{}
	return
}

func InstallAppToRouter(a *App, ur ufx.Router) {}
