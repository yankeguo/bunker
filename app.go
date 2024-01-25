package bunker

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"github.com/yankeguo/halt"
	"github.com/yankeguo/rg"
	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type App struct {
	db *gorm.DB
}

type AppOptions struct {
	fx.In

	DB *gorm.DB
}

func CreateApp(opts AppOptions) (app *App, err error) {
	app = &App{
		db: opts.DB,
	}
	return
}

func (a *App) currentUser(c ufx.Context) (token *model.Token, user *model.User, err error) {
	var cookie *http.Cookie

	if cookie, err = c.Req().Cookie("token"); err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			err = nil
		}
		return
	}

	db := dao.Use(a.db)

	if token, err = db.Token.Where(db.Token.ID.Eq(cookie.Value)).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
		}
		return
	}

	if user, err = db.User.Where(db.User.ID.Eq(token.UserID)).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
		}
		return
	}

	return
}

func (a *App) routeCurrentUser(c ufx.Context) {
	token, user := rg.Must2(a.currentUser(c))
	if user != nil {
		user.PasswordDigest = ""
	}
	c.JSON(map[string]any{
		"token": token,
		"user":  user,
	})
}

func (a *App) requireUser(c ufx.Context) (token *model.Token, user *model.User) {
	token, user = rg.Must2(a.currentUser(c))

	if user == nil || token == nil {
		halt.String("Not signed in")
		return
	}
	return
}

func (a *App) requireAdmin(c ufx.Context) (token *model.Token, user *model.User) {
	token, user = a.requireUser(c)

	if !user.IsAdmin {
		halt.String("Not admin")
		return
	}
	return
}

func (a *App) routeSignIn(c ufx.Context) {
	var data struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		UserAgent string `json:"header_user_agent"`
	}
	c.Bind(&data)

	db := dao.Use(a.db)

	// find user
	user := rg.Must(db.User.Where(db.User.ID.Eq(data.Username)).First())
	if !user.CheckPassword(data.Password) {
		halt.String("invalid password", halt.WithBadRequest())
		return
	}
	user.PasswordDigest = ""

	// delete history tokens
	rg.Must(db.Token.Where(db.Token.UserID.Eq(user.ID), db.Token.CreatedAt.Lte(time.Now().Add(-time.Hour*24*7))).Delete())

	// create token
	id := make([]byte, 32)
	rand.Read(id)

	token := &model.Token{
		ID:        hex.EncodeToString(id),
		UserID:    user.ID,
		UserAgent: data.UserAgent,
		CreatedAt: time.Now(),
		VisitedAt: time.Now(),
	}

	rg.Must0(db.Token.Create(token))

	cookie := &http.Cookie{
		Name:     "token",
		Value:    token.ID,
		MaxAge:   3600 * 24 * 7,
		Path:     "/",
		HttpOnly: !Debug("ui"),
	}

	c.Header().Set("Set-Cookie", cookie.String())

	c.JSON(map[string]any{
		"token": token,
		"user":  user,
	})
}

func (a *App) routeSignOut(c ufx.Context) {
	token, _ := a.requireUser(c)

	db := dao.Use(a.db)
	rg.Must(db.Token.Where(db.Token.ID.Eq(token.ID)).Delete())

	cookie := &http.Cookie{
		Name:     "token",
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: !Debug("ui"),
	}

	c.Header().Set("Set-Cookie", cookie.String())

	c.JSON(map[string]any{})
}

func InstallAppToRouter(a *App, ur ufx.Router) {
	ur.HandleFunc("/backend/current_user", a.routeCurrentUser)
	ur.HandleFunc("/backend/sign_in", a.routeSignIn)
	ur.HandleFunc("/backend/sign_out", a.routeSignOut)
}
