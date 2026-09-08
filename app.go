package bunker

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

const (
	// tokenTTL is the lifetime of a sign-in token
	tokenTTL = time.Hour * 24 * 7

	minPasswordLength = 6
	// maxPasswordLength is the maximum password length accepted by bcrypt
	maxPasswordLength = 72

	// signInRateLimit is the number of failed sign-in attempts allowed per
	// client within signInRateWindow
	signInRateLimit = 20
	// signInRateWindow is the window for counting failed sign-in attempts
	signInRateWindow = time.Minute * 5
)

// dummyPasswordDigest is compared against for unknown usernames, so sign-in
// takes a similar amount of time whether the username exists or not
var dummyPasswordDigest = rg.Must(model.CreateUserPassword("bunker-dummy-password"))

type App struct {
	db *gorm.DB

	uiOpts        uiOptions
	trustProxy    bool
	signInLimiter *rateLimiter
}

type uiOptions struct {
	SSHHost string `json:"ssh_host"`
	SSHPort string `json:"ssh_port"`
}

type AppOptions struct {
	fx.In

	DB   *gorm.DB
	Conf ufx.Conf
}

func CreateApp(opts AppOptions) (app *App, err error) {
	app = &App{
		db:            opts.DB,
		signInLimiter: newRateLimiter(signInRateLimit, signInRateWindow),
	}

	var raw struct {
		SSHHost string `json:"ssh_host"`
		SSHPort any    `json:"ssh_port"`
	}

	if err = opts.Conf.Bind(&raw, "ui"); err != nil {
		return
	}

	app.uiOpts.SSHHost = raw.SSHHost

	// accept both quoted ("8022") and plain (8022) values
	switch v := raw.SSHPort.(type) {
	case nil:
	case string:
		app.uiOpts.SSHPort = strings.TrimSpace(v)
	case float64:
		app.uiOpts.SSHPort = strconv.Itoa(int(v))
	default:
		err = fmt.Errorf("ui.ssh_port: unsupported value %v (%T)", v, v)
	}

	var proxyOpts struct {
		TrustProxy bool `json:"trust_proxy"`
	}

	if err = opts.Conf.Bind(&proxyOpts, "server"); err != nil {
		return
	}

	app.trustProxy = proxyOpts.TrustProxy

	return
}

func checkPassword(p string) error {
	if utf8.RuneCountInString(p) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	if len(p) > maxPasswordLength {
		return fmt.Errorf("password must be at most %d bytes", maxPasswordLength)
	}
	return nil
}

// clientIP returns the client address, using the last X-Forwarded-For entry
// when bunker runs behind a trusted proxy
func (a *App) clientIP(c ufx.Context) string {
	if a.trustProxy {
		if xff := c.Req().Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}

	if ip, _, err := net.SplitHostPort(c.Req().RemoteAddr); err == nil {
		return ip
	}

	return c.Req().RemoteAddr
}

func (a *App) isSecureRequest(c ufx.Context) bool {
	if c.Req().TLS != nil {
		return true
	}
	if a.trustProxy {
		return strings.EqualFold(strings.TrimSpace(c.Req().Header.Get("X-Forwarded-Proto")), "https")
	}
	return false
}

func (a *App) sessionCookie(c ufx.Context, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "token",
		Value:    value,
		MaxAge:   maxAge,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.isSecureRequest(c),
		SameSite: http.SameSiteLaxMode,
	}
}

// post only allows POST requests, so state-changing endpoints cannot be
// triggered by cross-site GET requests
func post(fn ufx.HandlerFunc) ufx.HandlerFunc {
	return func(c ufx.Context) {
		if c.Req().Method != http.MethodPost {
			c.Header().Set("Allow", http.MethodPost)
			halt.String("method not allowed", halt.WithStatusCode(http.StatusMethodNotAllowed))
			return
		}
		fn(c)
	}
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

	if token, err = db.Token.Where(
		db.Token.ID.Eq(cookie.Value),
		db.Token.CreatedAt.Gte(time.Now().Add(-tokenTTL)),
	).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
		}
		token = nil
		return
	}

	if user, err = db.User.Where(db.User.ID.Eq(token.UserID)).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
		}
		token = nil
		user = nil
		return
	}

	// a blocked user is treated as signed out
	if user.IsBlocked {
		token = nil
		user = nil
		return
	}

	if now := time.Now(); now.Sub(token.VisitedAt) > time.Minute {
		token.VisitedAt = now
		_, _ = db.Token.Where(db.Token.ID.Eq(token.ID)).UpdateColumnSimple(db.Token.VisitedAt.Value(now))
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
		halt.String("not signed in", halt.WithStatusCode(http.StatusUnauthorized))
		return
	}
	return
}

func (a *App) requireAdmin(c ufx.Context) (token *model.Token, user *model.User) {
	token, user = a.requireUser(c)

	if !user.IsAdmin {
		halt.String("not admin", halt.WithStatusCode(http.StatusForbidden))
		return
	}
	return
}

func (a *App) routeSignIn(c ufx.Context) {
	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	c.Bind(&data)

	data.Username = strings.TrimSpace(data.Username)

	if data.Username == "" || data.Password == "" {
		halt.String("username and password are required", halt.WithBadRequest())
		return
	}

	clientIP := a.clientIP(c)

	if ok, retryAfter := a.signInLimiter.Allowed(clientIP); !ok {
		c.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		halt.String("too many failed sign-in attempts, please try again later", halt.WithStatusCode(http.StatusTooManyRequests))
		return
	}

	db := dao.Use(a.db)

	user, err := db.User.Where(db.User.ID.Eq(data.Username)).First()

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		rg.Must0(err)
	}

	if user == nil {
		// burn the same amount of time as a wrong password, to avoid
		// leaking whether the username exists
		_ = model.CheckPasswordDigest(dummyPasswordDigest, data.Password)
		a.signInLimiter.Fail(clientIP)
		halt.String("invalid username or password", halt.WithBadRequest())
		return
	}

	if !user.CheckPassword(data.Password) {
		a.signInLimiter.Fail(clientIP)
		halt.String("invalid username or password", halt.WithBadRequest())
		return
	}

	if user.IsBlocked {
		a.signInLimiter.Fail(clientIP)
		halt.String("user is blocked", halt.WithBadRequest())
		return
	}

	a.signInLimiter.Reset(clientIP)

	// delete expired tokens
	rg.Must(db.Token.Where(db.Token.UserID.Eq(user.ID), db.Token.CreatedAt.Lte(time.Now().Add(-tokenTTL))).Delete())

	now := time.Now()

	rg.Must(db.User.Where(db.User.ID.Eq(user.ID)).UpdateColumnSimple(db.User.VisitedAt.Value(now)))
	user.VisitedAt = now

	// create token
	id := make([]byte, 32)
	rg.Must(rand.Read(id))

	token := &model.Token{
		ID:        hex.EncodeToString(id),
		UserID:    user.ID,
		UserAgent: c.Req().UserAgent(),
		CreatedAt: now,
		VisitedAt: now,
	}

	rg.Must0(db.Token.Create(token))

	c.Header().Set("Set-Cookie", a.sessionCookie(c, token.ID, int(tokenTTL.Seconds())).String())

	user.PasswordDigest = ""

	c.JSON(map[string]any{
		"token": token,
		"user":  user,
	})
}

func (a *App) routeSignOut(c ufx.Context) {
	token, _ := a.requireUser(c)

	db := dao.Use(a.db)
	rg.Must(db.Token.Where(db.Token.ID.Eq(token.ID)).Delete())

	c.Header().Set("Set-Cookie", a.sessionCookie(c, "", -1).String())

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

	data.DisplayName = strings.TrimSpace(data.DisplayName)
	if data.DisplayName == "" {
		data.DisplayName = "Unnamed"
	}

	if data.PublicKey == "" {
		halt.String("public key is required", halt.WithBadRequest())
		return
	}

	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(data.PublicKey))
	if err != nil {
		halt.String("invalid public key: "+err.Error(), halt.WithBadRequest())
		return
	}

	id := ssh.FingerprintSHA256(k)

	db := dao.Use(a.db)

	existing, err := db.Key.Where(db.Key.ID.Eq(id)).First()

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		rg.Must0(err)
	}

	if existing != nil {
		if existing.UserID != user.ID {
			halt.String("this public key is already registered by another user", halt.WithBadRequest())
			return
		}
		if existing.DisplayName != data.DisplayName {
			rg.Must(db.Key.Where(db.Key.ID.Eq(id)).UpdateColumnSimple(db.Key.DisplayName.Value(data.DisplayName)))
			existing.DisplayName = data.DisplayName
		}
		c.JSON(map[string]any{"key": existing})
		return
	}

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
		ID      string `json:"id"`
		Address string `json:"address"`
	}

	c.Bind(&data)

	data.ID = strings.TrimSpace(data.ID)
	data.Address = strings.TrimSpace(data.Address)

	if data.ID == "" {
		halt.String("server id is required", halt.WithBadRequest())
		return
	}

	if strings.ContainsAny(data.ID, " \t\r\n@") {
		halt.String("server id must not contain whitespace or @", halt.WithBadRequest())
		return
	}

	if data.Address == "" {
		halt.String("server address is required", halt.WithBadRequest())
		return
	}

	server := rg.Must(db.Server.Where(db.Server.ID.Eq(data.ID)).Assign(db.Server.Address.Value(data.Address)).FirstOrCreate())

	c.JSON(map[string]any{"server": server})
}

func (a *App) routeDeleteServer(c ufx.Context) {
	_, _ = a.requireAdmin(c)

	db := dao.Use(a.db)

	var data struct {
		ID string `json:"id"`
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
		ID       string `json:"id"`
		Password string `json:"password"`
	}
	c.Bind(&data)

	data.ID = strings.TrimSpace(data.ID)

	if !model.UserIDPattern.MatchString(data.ID) {
		halt.String("invalid username: must start with a lowercase letter, followed by at least 3 lowercase letters, digits, dot, underscore or hyphen", halt.WithBadRequest())
		return
	}

	if err := checkPassword(data.Password); err != nil {
		halt.Error(err, halt.WithBadRequest())
		return
	}

	user := &model.User{
		ID:        data.ID,
		CreatedAt: time.Now(),
		VisitedAt: time.Now(),
	}
	rg.Must0(user.SetPassword(data.Password))

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
		ID        string `json:"id"`
		IsAdmin   *bool  `json:"is_admin"`
		IsBlocked *bool  `json:"is_blocked"`
	}
	c.Bind(&data)

	if data.ID == "" {
		halt.String("user id is required", halt.WithBadRequest())
		return
	}

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
		UserID     string `json:"user_id"`
		ServerUser string `json:"server_user"`
		ServerID   string `json:"server_id"`
	}
	c.Bind(&data)

	data.UserID = strings.TrimSpace(data.UserID)
	data.ServerUser = strings.TrimSpace(data.ServerUser)
	data.ServerID = strings.TrimSpace(data.ServerID)

	if data.UserID == "" || data.ServerUser == "" || data.ServerID == "" {
		halt.String("user_id, server_user and server_id are required", halt.WithBadRequest())
		return
	}

	if _, err := db.User.Where(db.User.ID.Eq(data.UserID)).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			halt.String("user not found", halt.WithBadRequest())
			return
		}
		rg.Must0(err)
	}

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
		ID string `json:"id"`
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

	type grantMatcher struct {
		serverUser string
		matcher    *wildmatch.Wildmatch
	}

	matchers := make([]grantMatcher, 0, len(grants))

	for _, grant := range grants {
		matchers = append(matchers, grantMatcher{
			serverUser: grant.ServerUser,
			matcher: wildmatch.NewWildmatch(
				grant.ServerID,
				wildmatch.Basename,
				wildmatch.CaseFold,
			),
		})
	}

	grantedItems := []grantedItem{}

	for _, server := range servers {
		users := map[string]struct{}{}

		for _, m := range matchers {
			if m.matcher.Match(server.ID) {
				users[m.serverUser] = struct{}{}
			}
		}

		if len(users) == 0 {
			continue
		}

		names := make([]string, 0, len(users))
		for name := range users {
			names = append(names, name)
		}
		sort.Strings(names)

		grantedItems = append(grantedItems, grantedItem{
			ServerID:   server.ID,
			ServerUser: names[0],
		})
	}

	sort.SliceStable(grantedItems, func(i, j int) bool {
		return grantedItems[i].ServerID < grantedItems[j].ServerID
	})

	c.JSON(map[string]any{"granted_items": grantedItems})
}

func (a *App) routeUpdatePassword(c ufx.Context) {
	token, u := a.requireUser(c)

	var data struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	c.Bind(&data)

	if data.OldPassword == "" {
		halt.String("old password is required", halt.WithBadRequest())
		return
	}

	if err := checkPassword(data.NewPassword); err != nil {
		halt.Error(err, halt.WithBadRequest())
		return
	}

	if !u.CheckPassword(data.OldPassword) {
		halt.String("invalid old password", halt.WithBadRequest())
		return
	}

	rg.Must0(u.SetPassword(data.NewPassword))

	db := dao.Use(a.db)

	rg.Must(db.User.Where(db.User.ID.Eq(u.ID)).UpdateColumnSimple(db.User.PasswordDigest.Value(u.PasswordDigest)))

	// sign out every other session after a password change
	rg.Must(db.Token.Where(db.Token.UserID.Eq(u.ID), db.Token.ID.Neq(token.ID)).Delete())

	c.JSON(map[string]any{})
}

func InstallAppToRouter(a *App, ur ufx.Router) {
	ur.HandleFunc("/backend/ui_options", a.routeUIOptions)
	ur.HandleFunc("/backend/sign_in", post(a.routeSignIn))
	ur.HandleFunc("/backend/sign_out", post(a.routeSignOut))
	ur.HandleFunc("/backend/update_password", post(a.routeUpdatePassword))
	ur.HandleFunc("/backend/current_user", a.routeCurrentUser)
	ur.HandleFunc("/backend/granted_items", a.routeGrantedItems)
	ur.HandleFunc("/backend/keys", a.routeListKeys)
	ur.HandleFunc("/backend/keys/create", post(a.routeCreateKey))
	ur.HandleFunc("/backend/keys/delete", post(a.routeDeleteKey))
	ur.HandleFunc("/backend/servers", a.routeListServers)
	ur.HandleFunc("/backend/servers/create", post(a.routeCreateServer))
	ur.HandleFunc("/backend/servers/delete", post(a.routeDeleteServer))
	ur.HandleFunc("/backend/users", a.routeListUsers)
	ur.HandleFunc("/backend/users/create", post(a.routeCreateUser))
	ur.HandleFunc("/backend/users/update", post(a.routeUpdateUser))
	ur.HandleFunc("/backend/grants", a.routeListGrants)
	ur.HandleFunc("/backend/grants/create", post(a.routeCreateGrant))
	ur.HandleFunc("/backend/grants/delete", post(a.routeDeleteGrant))
}
