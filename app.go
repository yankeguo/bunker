package bunker

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"github.com/yankeguo/rg"
	"go.uber.org/fx"
	"go.uber.org/zap"
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

	// passwordRateLimit bounds guesses of the current password
	passwordRateLimit  = 10
	passwordRateWindow = time.Minute * 5

	// maxAPIBody is the largest JSON body a state-changing request may send
	maxAPIBody = 256 << 10
)

// dummyPasswordDigest is compared against for unknown usernames, so sign-in
// takes a similar amount of time whether the username exists or not
var dummyPasswordDigest = rg.Must(model.CreateUserPassword("bunker-dummy-password"))

type App struct {
	db      *gorm.DB
	log     *zap.SugaredLogger
	signers *Signers

	uiOpts          uiOptions
	trustProxy      bool
	signInLimiter   *rateLimiter
	passwordLimiter *rateLimiter
}

type uiOptions struct {
	SSHHost string `json:"ssh_host"`
	SSHPort string `json:"ssh_port"`
}

type AppOptions struct {
	fx.In

	DB      *gorm.DB
	Conf    Config
	Logger  *zap.SugaredLogger
	Signers *Signers
}

func CreateApp(opts AppOptions) (app *App, err error) {
	app = &App{
		db:              opts.DB,
		log:             opts.Logger,
		signers:         opts.Signers,
		signInLimiter:   newRateLimiter(signInRateLimit, signInRateWindow),
		passwordLimiter: newRateLimiter(passwordRateLimit, passwordRateWindow),
	}

	app.uiOpts.SSHHost = opts.Conf.UI.SSHHost
	app.uiOpts.SSHPort = opts.Conf.UI.SSHPort
	app.trustProxy = opts.Conf.Server.TrustProxy

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
func (a *App) clientIP(r *http.Request) string {
	if a.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}

	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}

	return r.RemoteAddr
}

func (a *App) isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if a.trustProxy {
		return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	}
	return false
}

func (a *App) sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "token",
		Value:    value,
		MaxAge:   maxAge,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	}
}

type apiHandler func(http.ResponseWriter, *http.Request) error

func (a *App) call(w http.ResponseWriter, r *http.Request, fn apiHandler) {
	defer func() {
		if rec := recover(); rec != nil {
			if a.log != nil {
				a.log.With("panic", fmt.Sprint(rec), "stack", string(debug.Stack())).Error("request panicked")
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "internal server error"})
		}
	}()
	if err := fn(w, r); err != nil {
		writeAPIError(a.log, w, err)
	}
}

func (a *App) get(fn apiHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		a.call(w, r, fn)
	}
}

func isJSONRequest(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

// post only accepts a JSON body. The route itself is registered as POST, so a
// cross-site form or a GET cannot run the handler.
func (a *App) post(fn apiHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !isJSONRequest(r) {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"message": "content type must be application/json"})
			return
		}
		r.Body = http.MaxBytesReader(nil, r.Body, maxAPIBody)
		a.call(w, r, fn)
	}
}

// sessionView is the sign-in record returned to the browser. The token id
// stays in the HttpOnly cookie and is not repeated in JSON.
type sessionView struct {
	UserID    string    `json:"user_id"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
	VisitedAt time.Time `json:"visited_at"`
}

func publishToken(token *model.Token) *sessionView {
	if token == nil {
		return nil
	}
	return &sessionView{
		UserID:    token.UserID,
		UserAgent: token.UserAgent,
		CreatedAt: token.CreatedAt,
		VisitedAt: token.VisitedAt,
	}
}

func publishUser(user *model.User) *model.User {
	if user == nil {
		return nil
	}
	user.PasswordDigest = ""
	return user
}

func (a *App) currentUser(r *http.Request) (token *model.Token, user *model.User, err error) {
	var cookie *http.Cookie

	if cookie, err = r.Cookie("token"); err != nil {
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

func (a *App) routeUIOptions(w http.ResponseWriter, _ *http.Request) error {
	writeJSON(w, http.StatusOK, a.uiOpts)
	return nil
}

func (a *App) routeCurrentUser(w http.ResponseWriter, r *http.Request) error {
	token, user, err := a.currentUser(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": publishToken(token),
		"user":  publishUser(user),
	})
	return nil
}

func (a *App) requireUser(r *http.Request) (token *model.Token, user *model.User, err error) {
	token, user, err = a.currentUser(r)
	if err != nil {
		return nil, nil, err
	}
	if user == nil || token == nil {
		return nil, nil, httpFail(http.StatusUnauthorized, "not signed in")
	}
	return token, user, nil
}

func (a *App) requireAdmin(r *http.Request) (token *model.Token, user *model.User, err error) {
	token, user, err = a.requireUser(r)
	if err != nil {
		return nil, nil, err
	}
	if !user.IsAdmin {
		return nil, nil, httpFail(http.StatusForbidden, "not admin")
	}
	return token, user, nil
}

func (a *App) routeSignIn(w http.ResponseWriter, r *http.Request) error {
	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &data); err != nil {
		return err
	}

	data.Username = strings.TrimSpace(data.Username)

	if data.Username == "" || data.Password == "" {
		return httpFail(http.StatusBadRequest, "username and password are required")
	}

	clientIP := a.clientIP(r)

	if ok, retryAfter := a.signInLimiter.Allowed(clientIP); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		return httpFail(http.StatusTooManyRequests, "too many failed sign-in attempts, please try again later")
	}

	db := dao.Use(a.db)

	user, err := db.User.Where(db.User.ID.Eq(data.Username)).First()

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if user == nil {
		// burn the same amount of time as a wrong password, to avoid
		// leaking whether the username exists
		_ = model.CheckPasswordDigest(dummyPasswordDigest, data.Password)
		a.signInLimiter.Fail(clientIP)
		a.log.With("username", data.Username, "ip", clientIP).Info("sign-in failed")
		return httpFail(http.StatusBadRequest, "invalid username or password")
	}

	if !user.CheckPassword(data.Password) {
		a.signInLimiter.Fail(clientIP)
		a.log.With("username", data.Username, "ip", clientIP).Info("sign-in failed")
		return httpFail(http.StatusBadRequest, "invalid username or password")
	}

	if user.IsBlocked {
		a.signInLimiter.Fail(clientIP)
		a.log.With("username", data.Username, "ip", clientIP).Info("sign-in rejected, user is blocked")
		return httpFail(http.StatusBadRequest, "user is blocked")
	}

	a.signInLimiter.Reset(clientIP)

	// delete expired tokens
	if _, err = db.Token.Where(db.Token.UserID.Eq(user.ID), db.Token.CreatedAt.Lte(time.Now().Add(-tokenTTL))).Delete(); err != nil {
		return err
	}

	now := time.Now()

	if _, err = db.User.Where(db.User.ID.Eq(user.ID)).UpdateColumnSimple(db.User.VisitedAt.Value(now)); err != nil {
		return err
	}
	user.VisitedAt = now

	// create token
	id := make([]byte, 32)
	if _, err = rand.Read(id); err != nil {
		return err
	}

	token := &model.Token{
		ID:        hex.EncodeToString(id),
		UserID:    user.ID,
		UserAgent: normalizeUserAgent(r.UserAgent()),
		CreatedAt: now,
		VisitedAt: now,
	}

	if err = db.Token.Create(token); err != nil {
		return err
	}

	http.SetCookie(w, a.sessionCookie(r, token.ID, int(tokenTTL.Seconds())))

	a.log.With("username", user.ID, "ip", clientIP).Info("sign-in succeeded")

	writeJSON(w, http.StatusOK, map[string]any{
		"token": publishToken(token),
		"user":  publishUser(user),
	})
	return nil
}

func (a *App) routeSignOut(w http.ResponseWriter, r *http.Request) error {
	token, _, err := a.requireUser(r)
	if err != nil {
		return err
	}

	db := dao.Use(a.db)
	if _, err = db.Token.Where(db.Token.ID.Eq(token.ID)).Delete(); err != nil {
		return err
	}

	http.SetCookie(w, a.sessionCookie(r, "", -1))

	writeJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (a *App) routeListKeys(w http.ResponseWriter, r *http.Request) error {
	_, user, err := a.requireUser(r)
	if err != nil {
		return err
	}

	db := dao.Use(a.db)

	keys, err := db.Key.Where(db.Key.UserID.Eq(user.ID)).Order(db.Key.CreatedAt.Desc()).Find()
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
	return nil
}

func (a *App) routeCreateKey(w http.ResponseWriter, r *http.Request) error {
	_, user, err := a.requireUser(r)
	if err != nil {
		return err
	}

	var data struct {
		DisplayName string `json:"display_name"`
		PublicKey   string `json:"public_key"`
	}
	if err = decodeJSON(r, &data); err != nil {
		return err
	}

	displayName, err := normalizeDisplayName(data.DisplayName)
	if err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}
	data.DisplayName = displayName

	k, err := parseUserPublicKey(data.PublicKey)
	if err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}

	id := ssh.FingerprintSHA256(k)

	db := dao.Use(a.db)

	existing, err := db.Key.Where(db.Key.ID.Eq(id)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if existing != nil {
		if existing.UserID != user.ID {
			return httpFail(http.StatusBadRequest, "this public key is already registered by another user")
		}
		if existing.DisplayName != data.DisplayName {
			if _, err = db.Key.Where(db.Key.ID.Eq(id)).UpdateColumnSimple(db.Key.DisplayName.Value(data.DisplayName)); err != nil {
				return err
			}
			existing.DisplayName = data.DisplayName
		}
		writeJSON(w, http.StatusOK, map[string]any{"key": existing})
		return nil
	}

	key := &model.Key{
		ID:          id,
		DisplayName: data.DisplayName,
		UserID:      user.ID,
		CreatedAt:   time.Now(),
	}
	if err = db.Key.Create(key); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"key": key})
	return nil
}

func (a *App) routeDeleteKey(w http.ResponseWriter, r *http.Request) error {
	_, user, err := a.requireUser(r)
	if err != nil {
		return err
	}

	var data struct {
		ID string `json:"id"`
	}
	if err = decodeJSON(r, &data); err != nil {
		return err
	}
	data.ID = strings.TrimSpace(data.ID)
	if data.ID == "" {
		return httpFail(http.StatusBadRequest, "key id is required")
	}

	db := dao.Use(a.db)
	if _, err = db.Key.Where(db.Key.ID.Eq(data.ID), db.Key.UserID.Eq(user.ID)).Delete(); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (a *App) routeListServers(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)
	servers, err := db.Server.Order(db.Server.ID).Find()
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"servers": servers})
	return nil
}

func (a *App) routeCreateServer(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)

	var data struct {
		ID      string `json:"id"`
		Address string `json:"address"`
	}
	if err := decodeJSON(r, &data); err != nil {
		return err
	}

	data.ID = strings.TrimSpace(data.ID)
	data.Address = strings.TrimSpace(data.Address)

	if err := validateServerID(data.ID); err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}

	address, err := normalizeServerAddress(data.Address)
	if err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}
	data.Address = address

	server, err := db.Server.Where(db.Server.ID.Eq(data.ID)).Assign(db.Server.Address.Value(data.Address)).FirstOrCreate()
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"server": server})
	return nil
}

func (a *App) routeDeleteServer(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)

	var data struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &data); err != nil {
		return err
	}
	data.ID = strings.TrimSpace(data.ID)
	if data.ID == "" {
		return httpFail(http.StatusBadRequest, "server id is required")
	}

	if _, err := db.HostKey.Where(db.HostKey.ServerID.Eq(data.ID)).Delete(); err != nil {
		return err
	}
	if _, err := db.Server.Where(db.Server.ID.Eq(data.ID)).Delete(); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (a *App) routeListUsers(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)
	users, err := db.User.Order(db.User.ID).Find()
	if err != nil {
		return err
	}
	for _, user := range users {
		publishUser(user)
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
	return nil
}

func (a *App) routeCreateUser(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)

	var data struct {
		ID       string `json:"id"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &data); err != nil {
		return err
	}

	data.ID = strings.TrimSpace(data.ID)

	if !model.UserIDPattern.MatchString(data.ID) {
		return httpFail(http.StatusBadRequest, "invalid username: must start with a lowercase letter, followed by at least 3 lowercase letters, digits, dot, underscore or hyphen")
	}

	if err := checkPassword(data.Password); err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}

	existing, err := db.User.Where(db.User.ID.Eq(data.ID)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if existing == nil {
		user := &model.User{
			ID:        data.ID,
			CreatedAt: time.Now(),
			VisitedAt: time.Now(),
		}
		if err = user.SetPassword(data.Password); err != nil {
			return err
		}
		if err = db.User.Create(user); err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": publishUser(user)})
		return nil
	}

	if !existing.CheckPassword(data.Password) {
		if err = existing.SetPassword(data.Password); err != nil {
			return err
		}
		if _, err = db.User.Where(db.User.ID.Eq(existing.ID)).UpdateColumnSimple(db.User.PasswordDigest.Value(existing.PasswordDigest)); err != nil {
			return err
		}
		if _, err = db.Token.Where(db.Token.UserID.Eq(existing.ID)).Delete(); err != nil {
			return err
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": publishUser(existing)})
	return nil
}

func (a *App) routeUpdateUser(w http.ResponseWriter, r *http.Request) error {
	_, u, err := a.requireAdmin(r)
	if err != nil {
		return err
	}

	db := dao.Use(a.db)

	var data struct {
		ID        string `json:"id"`
		IsAdmin   *bool  `json:"is_admin"`
		IsBlocked *bool  `json:"is_blocked"`
	}
	if err = decodeJSON(r, &data); err != nil {
		return err
	}

	if data.ID == "" {
		return httpFail(http.StatusBadRequest, "user id is required")
	}

	if u.ID == data.ID {
		return httpFail(http.StatusBadRequest, "cannot edit self")
	}

	// count and update in one transaction so two admins cannot demote each
	// other at the same time and leave the system with nobody in charge
	err = db.Transaction(func(tx *dao.Query) error {
		target, err := tx.User.Where(tx.User.ID.Eq(data.ID)).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return httpFail(http.StatusBadRequest, "user not found")
			}
			return err
		}

		willBeAdmin := target.IsAdmin
		if data.IsAdmin != nil {
			willBeAdmin = *data.IsAdmin
		}
		willBeBlocked := target.IsBlocked
		if data.IsBlocked != nil {
			willBeBlocked = *data.IsBlocked
		}
		if target.IsAdmin && !target.IsBlocked && (!willBeAdmin || willBeBlocked) {
			admins, err := tx.User.Where(tx.User.IsAdmin.Is(true), tx.User.IsBlocked.Is(false)).Find()
			if err != nil {
				return err
			}
			if len(admins) <= 1 {
				return httpFail(http.StatusBadRequest, "cannot remove the last admin")
			}
		}

		var assigns []field.AssignExpr
		if data.IsAdmin != nil {
			assigns = append(assigns, tx.User.IsAdmin.Value(*data.IsAdmin))
		}
		if data.IsBlocked != nil {
			assigns = append(assigns, tx.User.IsBlocked.Value(*data.IsBlocked))
		}
		if len(assigns) != 0 {
			if _, err = tx.User.Where(tx.User.ID.Eq(data.ID)).UpdateColumnSimple(assigns...); err != nil {
				return err
			}
		}
		if willBeBlocked {
			if _, err = tx.Token.Where(tx.Token.UserID.Eq(data.ID)).Delete(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (a *App) routeListGrants(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))

	db := dao.Use(a.db)
	grants, err := db.Grant.Where(db.Grant.UserID.Eq(userID)).Order(db.Grant.CreatedAt).Find()
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"grants": grants})
	return nil
}

func (a *App) routeCreateGrant(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)

	var data struct {
		UserID     string `json:"user_id"`
		ServerUser string `json:"server_user"`
		ServerID   string `json:"server_id"`
	}
	if err := decodeJSON(r, &data); err != nil {
		return err
	}

	data.UserID = strings.TrimSpace(data.UserID)
	data.ServerUser = strings.TrimSpace(data.ServerUser)
	data.ServerID = strings.TrimSpace(data.ServerID)

	if err := validateGrantPattern("server_user", data.ServerUser); err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}
	if err := validateGrantPattern("server_id", data.ServerID); err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}
	if data.UserID == "" {
		return httpFail(http.StatusBadRequest, "user_id is required")
	}

	if _, err := db.User.Where(db.User.ID.Eq(data.UserID)).First(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpFail(http.StatusBadRequest, "user not found")
		}
		return err
	}

	digest := sha256.Sum256([]byte(data.UserID + "::" + data.ServerUser + "@" + data.ServerID))
	id := hex.EncodeToString(digest[:])

	grant := &model.Grant{
		ID:         id,
		UserID:     data.UserID,
		ServerUser: data.ServerUser,
		ServerID:   data.ServerID,
	}

	if err := db.Grant.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(grant); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"grant": grant})
	return nil
}

func (a *App) routeDeleteGrant(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)

	var data struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &data); err != nil {
		return err
	}
	data.ID = strings.TrimSpace(data.ID)
	if data.ID == "" {
		return httpFail(http.StatusBadRequest, "grant id is required")
	}

	if _, err := db.Grant.Where(db.Grant.ID.Eq(data.ID)).Delete(); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (a *App) routeGrantedItems(w http.ResponseWriter, r *http.Request) error {
	_, u, err := a.requireUser(r)
	if err != nil {
		return err
	}

	db := dao.Use(a.db)
	grants, err := db.Grant.Where(db.Grant.UserID.Eq(u.ID)).Find()
	if err != nil {
		return err
	}
	servers, err := db.Server.Find()
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"granted_items": expandGrantedItems(grants, servers)})
	return nil
}

func (a *App) routeUpdatePassword(w http.ResponseWriter, r *http.Request) error {
	token, u, err := a.requireUser(r)
	if err != nil {
		return err
	}

	var data struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err = decodeJSON(r, &data); err != nil {
		return err
	}

	if data.OldPassword == "" {
		return httpFail(http.StatusBadRequest, "old password is required")
	}

	if err = checkPassword(data.NewPassword); err != nil {
		return httpFail(http.StatusBadRequest, err.Error())
	}

	if ok, retryAfter := a.passwordLimiter.Allowed(u.ID); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		return httpFail(http.StatusTooManyRequests, "too many failed attempts, please try again later")
	}

	if !u.CheckPassword(data.OldPassword) {
		a.passwordLimiter.Fail(u.ID)
		return httpFail(http.StatusBadRequest, "invalid old password")
	}
	a.passwordLimiter.Reset(u.ID)

	if err = u.SetPassword(data.NewPassword); err != nil {
		return err
	}

	db := dao.Use(a.db)
	if _, err = db.User.Where(db.User.ID.Eq(u.ID)).UpdateColumnSimple(db.User.PasswordDigest.Value(u.PasswordDigest)); err != nil {
		return err
	}

	// sign out every other session after a password change
	if _, err = db.Token.Where(db.Token.UserID.Eq(u.ID), db.Token.ID.Neq(token.ID)).Delete(); err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (a *App) routeAuthorizedKeys(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write([]byte(a.signers.AuthorizedKeys))
	return err
}

func (a *App) routeListHostKeys(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	db := dao.Use(a.db)
	keys, err := db.HostKey.Order(db.HostKey.ServerID, db.HostKey.KeyType).Find()
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, map[string]any{"host_keys": keys})
	return nil
}

func (a *App) routeDeleteHostKeys(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := a.requireAdmin(r); err != nil {
		return err
	}

	var data struct {
		ServerID string `json:"server_id"`
	}
	if err := decodeJSON(r, &data); err != nil {
		return err
	}
	data.ServerID = strings.TrimSpace(data.ServerID)
	if data.ServerID == "" {
		return httpFail(http.StatusBadRequest, "server id is required")
	}

	db := dao.Use(a.db)
	if _, err := db.HostKey.Where(db.HostKey.ServerID.Eq(data.ServerID)).Delete(); err != nil {
		return err
	}
	a.log.With("server_id", data.ServerID).Info("ssh host keys reset")

	writeJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (a *App) mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /backend/ui_options", a.get(a.routeUIOptions))
	mux.HandleFunc("POST /backend/sign_in", a.post(a.routeSignIn))
	mux.HandleFunc("POST /backend/sign_out", a.post(a.routeSignOut))
	mux.HandleFunc("POST /backend/update_password", a.post(a.routeUpdatePassword))
	mux.HandleFunc("GET /backend/current_user", a.get(a.routeCurrentUser))
	mux.HandleFunc("GET /backend/granted_items", a.get(a.routeGrantedItems))
	mux.HandleFunc("GET /backend/authorized_keys", a.get(a.routeAuthorizedKeys))
	mux.HandleFunc("GET /backend/keys", a.get(a.routeListKeys))
	mux.HandleFunc("POST /backend/keys/create", a.post(a.routeCreateKey))
	mux.HandleFunc("POST /backend/keys/delete", a.post(a.routeDeleteKey))
	mux.HandleFunc("GET /backend/servers", a.get(a.routeListServers))
	mux.HandleFunc("POST /backend/servers/create", a.post(a.routeCreateServer))
	mux.HandleFunc("POST /backend/servers/delete", a.post(a.routeDeleteServer))
	mux.HandleFunc("GET /backend/host_keys", a.get(a.routeListHostKeys))
	mux.HandleFunc("POST /backend/host_keys/delete", a.post(a.routeDeleteHostKeys))
	mux.HandleFunc("GET /backend/users", a.get(a.routeListUsers))
	mux.HandleFunc("POST /backend/users/create", a.post(a.routeCreateUser))
	mux.HandleFunc("POST /backend/users/update", a.post(a.routeUpdateUser))
	mux.HandleFunc("GET /backend/grants", a.get(a.routeListGrants))
	mux.HandleFunc("POST /backend/grants/create", a.post(a.routeCreateGrant))
	mux.HandleFunc("POST /backend/grants/delete", a.post(a.routeDeleteGrant))
}
