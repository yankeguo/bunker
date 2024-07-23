package bunker

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/git-lfs/wildmatch"
	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"github.com/yankeguo/halt"
	"github.com/yankeguo/rg"
	"github.com/yankeguo/ufx"
	"go.uber.org/fx"
	"golang.org/x/crypto/ssh"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type App struct {
	db *gorm.DB

	uiOpts uiOptions
}

type uiOptions struct {
	SSHHost string `json:"ssh_host"`
	SSHPort int    `json:"ssh_port"`
}

type AppOptions struct {
	fx.In

	DB   *gorm.DB
	Conf ufx.Conf
}

func CreateApp(opts AppOptions) (app *App, err error) {
	app = &App{
		db: opts.DB,
	}
	err = opts.Conf.Bind(&app.uiOpts, "ui")
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

func (a *App) routeUIOptions(c ufx.Context) {
	c.JSON(a.uiOpts)
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

	// must not blocked
	if user.IsBlocked {
		halt.String("blocked", halt.WithBadRequest())
		return
	}

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

func (a *App) routeListKeys(c ufx.Context) {
	_, user := a.requireUser(c)

	db := dao.Use(a.db)

	keys := rg.Must(db.Key.Where(db.Key.UserID.Eq(user.ID)).Find())

	c.JSON(map[string]any{"keys": keys})
}

func (a *App) routeCreateKey(c ufx.Context) {
	_, user := a.requireUser(c)

	var data struct {
		DisplayName string `json:"display_name"`
		PublicKey   string `json:"public_key"`
	}
	c.Bind(&data)

	if data.DisplayName == "" {
		data.DisplayName = "Unnamed"
	}

	if data.PublicKey == "" {
		halt.String("public key is required", halt.WithBadRequest())
		return
	}

	k, _, _, _ := rg.Must4(ssh.ParseAuthorizedKey([]byte(data.PublicKey)))

	id := ssh.FingerprintSHA256(k)

	db := dao.Use(a.db)

	key := &model.Key{
		ID:          id,
		DisplayName: data.DisplayName,
		UserID:      user.ID,
		CreatedAt:   time.Now(),
	}

	rg.Must0(db.Key.Create(key))

	c.JSON(map[string]any{"key": key})
}

func (a *App) routeDeleteKey(c ufx.Context) {
	_, user := a.requireUser(c)

	var data struct {
		ID string `json:"id"`
	}
	c.Bind(&data)

	db := dao.Use(a.db)

	rg.Must(db.Key.Where(db.Key.ID.Eq(data.ID), db.Key.UserID.Eq(user.ID)).Delete())

	c.JSON(map[string]any{})
}

func (a *App) routeListServers(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	servers := rg.Must(db.Server.Find())

	c.JSON(map[string]any{"servers": servers})
}

func (a *App) routeCreateServer(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	var data struct {
		ID      string `json:"id" validate:"required"`
		Address string `json:"address" validate:"required"`
	}

	c.Bind(&data)

	server := rg.Must(db.Server.Where(db.Server.ID.Eq(data.ID)).Assign(db.Server.Address.Value(data.Address)).FirstOrCreate())

	c.JSON(map[string]any{"server": server})
}

func (a *App) routeDeleteServer(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	var data struct {
		ID string `json:"id" validate:"required"`
	}

	c.Bind(&data)

	rg.Must(db.Server.Where(db.Server.ID.Eq(data.ID)).Delete())

	c.JSON(map[string]any{})
}

func (a *App) routeListUsers(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	users := rg.Must(db.User.Find())

	c.JSON(map[string]any{"users": users})
}

func (a *App) routeCreateUser(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	var data struct {
		ID       string `json:"id" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	c.Bind(&data)

	user := &model.User{
		ID:        data.ID,
		CreatedAt: time.Now(),
		VisitedAt: time.Now(),
	}
	user.SetPassword(data.Password)

	rg.Must0(db.User.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"password_digest"}),
	}).Create(user))

	c.JSON(map[string]any{"user": user})
}

func (a *App) routeUpdateUser(c ufx.Context) {
	_, u := a.requireAdmin(c)

	db := dao.Use(a.db)

	var data struct {
		ID        string `json:"id" validate:"required"`
		IsAdmin   *bool  `json:"is_admin"`
		IsBlocked *bool  `json:"is_blocked"`
	}
	c.Bind(&data)

	if u.ID == data.ID {
		halt.String("cannot edit self", halt.WithBadRequest())
		return
	}

	var assigns []field.AssignExpr

	if data.IsAdmin != nil {
		assigns = append(assigns, db.User.IsAdmin.Value(*data.IsAdmin))
	}

	if data.IsBlocked != nil {
		assigns = append(assigns, db.User.IsBlocked.Value(*data.IsBlocked))
	}

	if len(assigns) != 0 {
		rg.Must(db.User.Where(db.User.ID.Eq(data.ID)).UpdateColumnSimple(assigns...))
	}

	c.JSON(map[string]any{})
}

func (a *App) routeListGrants(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	var data struct {
		UserID string `json:"user_id"`
	}

	c.Bind(&data)

	db := dao.Use(a.db)

	grants := rg.Must(db.Grant.Where(db.Grant.UserID.Eq(data.UserID)).Find())

	c.JSON(map[string]any{"grants": grants})
}

func (a *App) routeCreateGrant(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	var data struct {
		UserID     string `json:"user_id" validate:"required"`
		ServerUser string `json:"server_user" validate:"required"`
		ServerID   string `json:"server_id" validate:"required"`
	}
	c.Bind(&data)

	digest := sha256.Sum256([]byte(data.UserID + "::" + data.ServerUser + "@" + data.ServerID))
	id := hex.EncodeToString(digest[:])

	grant := &model.Grant{
		ID:         id,
		UserID:     data.UserID,
		ServerUser: data.ServerUser,
		ServerID:   data.ServerID,
	}

	rg.Must0(db.Grant.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(grant))

	c.JSON(map[string]any{"grant": grant})
}

func (a *App) routeDeleteGrant(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	var data struct {
		ID string `json:"id" validate:"required"`
	}

	c.Bind(&data)

	rg.Must(db.Grant.Where(db.Grant.ID.Eq(data.ID)).Delete())

	c.JSON(map[string]any{})
}

func (a *App) routeGrantedItems(c ufx.Context) {
	_, u := a.requireUser(c)

	db := dao.Use(a.db)

	grants := rg.Must(db.Grant.Where(db.Grant.UserID.Eq(u.ID)).Find())

	servers := rg.Must(db.Server.Find())

	type grantedItem struct {
		ServerUser string `json:"server_user"`
		ServerID   string `json:"server_id"`
	}

	m := map[string][]string{}

	for _, grant := range grants {
		matcher := wildmatch.NewWildmatch(
			grant.ServerID,
			wildmatch.Basename,
			wildmatch.CaseFold,
		)

		for _, server := range servers {
			if matcher.Match(server.ID) {
				m[server.ID] = append(m[server.ID], grant.ServerUser)
			}
		}
	}

	grantedItems := []grantedItem{}

	for serverID, serverUsers := range m {
		grantedItems = append(grantedItems, grantedItem{
			ServerID:   serverID,
			ServerUser: serverUsers[0],
		})
	}

	sort.SliceStable(grantedItems, func(i, j int) bool {
		return grantedItems[i].ServerID < grantedItems[j].ServerID
	})

	c.JSON(map[string]any{"granted_items": grantedItems})
}

func (a *App) routeUpdatePassword(c ufx.Context) {
	_, u := a.requireUser(c)

	var data struct {
		OldPassword string `json:"old_password" validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=6"`
	}

	c.Bind(&data)

	if !u.CheckPassword(data.OldPassword) {
		halt.String("invalid old password", halt.WithBadRequest())
		return
	}

	u.SetPassword(data.NewPassword)

	db := dao.Use(a.db)

	rg.Must(db.User.Where(db.User.ID.Eq(u.ID)).UpdateColumnSimple(db.User.PasswordDigest.Value(u.PasswordDigest)))

	c.JSON(map[string]any{})
}

func InstallAppToRouter(a *App, ur ufx.Router) {
	ur.HandleFunc("/backend/ui_options", a.routeUIOptions)
	ur.HandleFunc("/backend/sign_in", a.routeSignIn)
	ur.HandleFunc("/backend/sign_out", a.routeSignOut)
	ur.HandleFunc("/backend/update_password", a.routeUpdatePassword)
	ur.HandleFunc("/backend/current_user", a.routeCurrentUser)
	ur.HandleFunc("/backend/granted_items", a.routeGrantedItems)
	ur.HandleFunc("/backend/keys", a.routeListKeys)
	ur.HandleFunc("/backend/keys/create", a.routeCreateKey)
	ur.HandleFunc("/backend/keys/delete", a.routeDeleteKey)
	ur.HandleFunc("/backend/servers", a.routeListServers)
	ur.HandleFunc("/backend/servers/create", a.routeCreateServer)
	ur.HandleFunc("/backend/servers/delete", a.routeDeleteServer)
	ur.HandleFunc("/backend/users", a.routeListUsers)
	ur.HandleFunc("/backend/users/create", a.routeCreateUser)
	ur.HandleFunc("/backend/users/update", a.routeUpdateUser)
	ur.HandleFunc("/backend/grants", a.routeListGrants)
	ur.HandleFunc("/backend/grants/create", a.routeCreateGrant)
	ur.HandleFunc("/backend/grants/delete", a.routeDeleteGrant)
}
