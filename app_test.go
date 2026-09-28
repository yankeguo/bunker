package bunker

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
)

func TestCheckPassword(t *testing.T) {
	if err := checkPassword(strings.Repeat("a", 72)); err != nil {
		t.Fatal(err)
	}
	if err := checkPassword(strings.Repeat("é", 6)); err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"short", strings.Repeat("é", 5), strings.Repeat("a", 73)} {
		if err := checkPassword(password); err == nil {
			t.Fatalf("checkPassword(%q) succeeded", password)
		}
	}
}

func TestWouldRemoveLastAdmin(t *testing.T) {
	if !wouldRemoveLastAdmin(1, true, false) {
		t.Fatal("expected the last active admin to be protected")
	}
	if wouldRemoveLastAdmin(2, true, false) {
		t.Fatal("demoting one of two admins should be allowed")
	}
	if wouldRemoveLastAdmin(1, true, true) || wouldRemoveLastAdmin(1, false, false) {
		t.Fatal("a change that keeps or does not touch an active admin should be allowed")
	}
}

func TestClientIP(t *testing.T) {
	app := &App{}
	req := httpRequest("10.0.0.8:1234")
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := app.clientIP(req); got != "10.0.0.8" {
		t.Fatalf("untrusted proxy ip = %s", got)
	}

	app.trustProxy = true
	if got := app.clientIP(req); got != "5.6.7.8" {
		t.Fatalf("trusted proxy ip = %s", got)
	}
	req.Header.Set("X-Forwarded-For", "1.2.3.4, not-an-ip")
	if got := app.clientIP(req); got != "10.0.0.8" {
		t.Fatalf("invalid forwarded ip = %s", got)
	}
	req.Header.Set("X-Forwarded-For", "")
	req.RemoteAddr = "[::1]:9"
	if got := app.clientIP(req); got != "::1" {
		t.Fatalf("ipv6 remote = %s", got)
	}
}

func TestSessionCookie(t *testing.T) {
	app := &App{}
	req := httpRequest("127.0.0.1:1")
	cookie := app.sessionCookie(req, "abc", 10)
	if cookie.Name != "token" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Secure || cookie.Path != "/" {
		t.Fatalf("cookie = %+v", cookie)
	}

	req.Header.Set("X-Forwarded-Proto", "https")
	if app.sessionCookie(req, "abc", 10).Secure {
		t.Fatal("forwarded proto was trusted without trust_proxy")
	}
	app.trustProxy = true
	if !app.sessionCookie(req, "abc", 10).Secure {
		t.Fatal("expected a secure cookie behind a trusted https proxy")
	}
	req.Header.Del("X-Forwarded-Proto")
	req.TLS = &tls.ConnectionState{}
	if !app.isSecureRequest(req) {
		t.Fatal("expected a TLS request to be secure")
	}
}

func TestUIOptionsAndCurrentUser(t *testing.T) {
	cfg := Config{}
	cfg.UI.SSHHost = "bunker.example"
	cfg.UI.SSHPort = "8022"
	_, h := newTestApp(t, cfg)

	rr := doJSON(t, h, http.MethodGet, "/backend/ui_options", "", "")
	requireStatus(t, rr, http.StatusOK)
	body := decodeBody(t, rr)
	if body["ssh_host"] != "bunker.example" || body["ssh_port"] != "8022" {
		t.Fatalf("ui options = %#v", body)
	}

	rr = doJSON(t, h, http.MethodGet, "/backend/current_user", "", "")
	requireStatus(t, rr, http.StatusOK)
	body = decodeBody(t, rr)
	if body["user"] != nil || body["token"] != nil {
		t.Fatalf("anonymous session = %#v", body)
	}
}

func TestSignInSession(t *testing.T) {
	app, h := newTestApp(t, Config{})
	insertUser(t, app.db, "alice", "secret1", true, false)
	insertUser(t, app.db, "bob", "secret1", false, true)

	rr := signIn(t, h, "nobody", "secret1")
	requireStatus(t, rr, http.StatusBadRequest)
	if !strings.Contains(rr.Body.String(), "invalid username or password") {
		t.Fatalf("body = %s", rr.Body.String())
	}

	rr = signIn(t, h, "alice", "wrong")
	requireStatus(t, rr, http.StatusBadRequest)

	rr = signIn(t, h, "bob", "secret1")
	requireStatus(t, rr, http.StatusBadRequest)
	if !strings.Contains(rr.Body.String(), "blocked") {
		t.Fatalf("body = %s", rr.Body.String())
	}

	rr = signIn(t, h, " alice ", "secret1")
	requireStatus(t, rr, http.StatusOK)
	if strings.Contains(rr.Body.String(), "$2") {
		t.Fatalf("password digest leaked: %s", rr.Body.String())
	}
	cookie := tokenCookie(t, rr)
	if !cookie.HttpOnly || cookie.MaxAge <= 0 {
		t.Fatalf("cookie = %+v", cookie)
	}
	body := decodeBody(t, rr)
	user := body["user"].(map[string]any)
	if user["id"] != "alice" || user["is_admin"] != true {
		t.Fatalf("user = %#v", user)
	}
	rr = doJSON(t, h, http.MethodPost, "/backend/sign_in", `{"username":"alice","password":"secret1","extra":true}`, "")
	requireStatus(t, rr, http.StatusOK)

	uaReq := httptest.NewRequest(http.MethodPost, "/backend/sign_in", strings.NewReader(`{"username":"alice","password":"secret1"}`))
	uaReq.Header.Set("Content-Type", "application/json")
	uaReq.Header.Set("User-Agent", "curl/8\r\nX")
	uaRR := httptest.NewRecorder()
	h.ServeHTTP(uaRR, uaReq)
	requireStatus(t, uaRR, http.StatusOK)
	if got := decodeBody(t, uaRR)["token"].(map[string]any)["user_agent"]; got != "curl/8  X" {
		t.Fatalf("user agent = %#v", got)
	}

	current := doJSON(t, h, http.MethodGet, "/backend/current_user", "", cookieHeader(cookie))
	requireStatus(t, current, http.StatusOK)
	if decodeBody(t, current)["user"].(map[string]any)["id"] != "alice" {
		t.Fatalf("current user = %s", current.Body.String())
	}

	out := doJSON(t, h, http.MethodPost, "/backend/sign_out", `{}`, cookieHeader(cookie))
	requireStatus(t, out, http.StatusOK)
	cleared := tokenCookie(t, out)
	if cleared.MaxAge >= 0 {
		t.Fatalf("sign-out cookie = %+v", cleared)
	}
	rr = doJSON(t, h, http.MethodGet, "/backend/keys", "", cookieHeader(cookie))
	requireStatus(t, rr, http.StatusUnauthorized)
}

func TestSignInRateLimitUsesClientIP(t *testing.T) {
	app, h := newTestApp(t, Config{})
	app.trustProxy = true
	app.signInLimiter = newRateLimiter(1, time.Minute)
	insertUser(t, app.db, "alice", "secret1", false, false)

	rr := postSignInFrom(t, h, "alice", "wrong", "9.9.9.9:1", "1.2.3.4")
	requireStatus(t, rr, http.StatusBadRequest)
	rr = postSignInFrom(t, h, "alice", "secret1", "9.9.9.9:1", "1.2.3.4")
	requireStatus(t, rr, http.StatusTooManyRequests)
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	rr = postSignInFrom(t, h, "alice", "wrong", "9.9.9.9:1", "8.8.8.8")
	requireStatus(t, rr, http.StatusBadRequest)

	app.trustProxy = false
	app.signInLimiter = newRateLimiter(1, time.Minute)
	rr = postSignInFrom(t, h, "alice", "wrong", "9.9.9.9:1", "1.1.1.1")
	requireStatus(t, rr, http.StatusBadRequest)
	rr = postSignInFrom(t, h, "alice", "wrong", "9.9.9.9:1", "2.2.2.2")
	requireStatus(t, rr, http.StatusTooManyRequests)
}

func TestUpdatePasswordRevokesOtherSessions(t *testing.T) {
	app, h := newTestApp(t, Config{})
	insertUser(t, app.db, "alice", "secret1", false, false)
	app.passwordLimiter = newRateLimiter(1, time.Minute)

	first := tokenCookie(t, signIn(t, h, "alice", "secret1"))
	second := tokenCookie(t, signIn(t, h, "alice", "secret1"))

	rr := doJSON(t, h, http.MethodPost, "/backend/update_password", `{"old_password":"nope","new_password":"secret2"}`, cookieHeader(second))
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/update_password", `{"old_password":"secret1","new_password":"secret2"}`, cookieHeader(second))
	requireStatus(t, rr, http.StatusTooManyRequests)

	app.passwordLimiter.Reset("alice")
	rr = doJSON(t, h, http.MethodPost, "/backend/update_password", `{"old_password":"","new_password":"secret2"}`, cookieHeader(second))
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/update_password", `{"old_password":"secret1","new_password":"short"}`, cookieHeader(second))
	requireStatus(t, rr, http.StatusBadRequest)

	rr = doJSON(t, h, http.MethodPost, "/backend/update_password", `{"old_password":"secret1","new_password":"secret2"}`, cookieHeader(second))
	requireStatus(t, rr, http.StatusOK)

	rr = doJSON(t, h, http.MethodGet, "/backend/current_user", "", cookieHeader(first))
	requireStatus(t, rr, http.StatusOK)
	if decodeBody(t, rr)["user"] != nil {
		t.Fatalf("old session still active: %s", rr.Body.String())
	}
	rr = doJSON(t, h, http.MethodGet, "/backend/current_user", "", cookieHeader(second))
	requireStatus(t, rr, http.StatusOK)
	if decodeBody(t, rr)["user"] == nil {
		t.Fatal("current session was revoked")
	}
	rr = signIn(t, h, "alice", "secret1")
	requireStatus(t, rr, http.StatusBadRequest)
	requireStatus(t, signIn(t, h, "alice", "secret2"), http.StatusOK)
}

func TestKeysAPI(t *testing.T) {
	app, h := newTestApp(t, Config{})
	insertUser(t, app.db, "alice", "secret1", false, false)
	insertUser(t, app.db, "bob", "secret1", false, false)
	alice := cookieHeader(tokenCookie(t, signIn(t, h, "alice", "secret1")))
	bob := cookieHeader(tokenCookie(t, signIn(t, h, "bob", "secret1")))

	rr := doJSON(t, h, http.MethodGet, "/backend/keys", "", "")
	requireStatus(t, rr, http.StatusUnauthorized)

	line := authorizedLine(testPublicKey(t))
	rr = doJSON(t, h, http.MethodPost, "/backend/keys/create", `{"display_name":"  ","public_key":"`+line+`"}`, alice)
	requireStatus(t, rr, http.StatusOK)
	key := decodeBody(t, rr)["key"].(map[string]any)
	if key["display_name"] != "Unnamed" {
		t.Fatalf("key = %#v", key)
	}
	id := key["id"].(string)

	rr = doJSON(t, h, http.MethodPost, "/backend/keys/create", `{"display_name":"laptop","public_key":"`+line+`"}`, alice)
	requireStatus(t, rr, http.StatusOK)
	if decodeBody(t, rr)["key"].(map[string]any)["display_name"] != "laptop" {
		t.Fatalf("renamed = %s", rr.Body.String())
	}

	rr = doJSON(t, h, http.MethodPost, "/backend/keys/create", `{"display_name":"stolen","public_key":"`+line+`"}`, bob)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = doJSON(t, h, http.MethodPost, "/backend/keys/create", `{"public_key":"`+line+`\n`+line+`"}`, alice)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = doJSON(t, h, http.MethodGet, "/backend/keys", "", alice)
	requireStatus(t, rr, http.StatusOK)
	if keys := decodeBody(t, rr)["keys"].([]any); len(keys) != 1 {
		t.Fatalf("keys = %#v", keys)
	}

	rr = doJSON(t, h, http.MethodPost, "/backend/keys/delete", `{"id":"`+id+`"}`, bob)
	requireStatus(t, rr, http.StatusOK)
	rr = doJSON(t, h, http.MethodGet, "/backend/keys", "", alice)
	if keys := decodeBody(t, rr)["keys"].([]any); len(keys) != 1 {
		t.Fatal("another user deleted the key")
	}

	rr = doJSON(t, h, http.MethodPost, "/backend/keys/delete", `{"id":""}`, alice)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/keys/delete", `{"id":"`+id+`"}`, alice)
	requireStatus(t, rr, http.StatusOK)
	rr = doJSON(t, h, http.MethodGet, "/backend/keys", "", alice)
	if keys := decodeBody(t, rr)["keys"].([]any); len(keys) != 0 {
		t.Fatalf("keys after delete = %#v", keys)
	}
}

func TestServersAPI(t *testing.T) {
	app, h := newTestApp(t, Config{})
	insertUser(t, app.db, "alice", "secret1", true, false)
	insertUser(t, app.db, "bob", "secret1", false, false)
	admin := cookieHeader(tokenCookie(t, signIn(t, h, "alice", "secret1")))
	user := cookieHeader(tokenCookie(t, signIn(t, h, "bob", "secret1")))

	rr := doJSON(t, h, http.MethodGet, "/backend/servers", "", user)
	requireStatus(t, rr, http.StatusForbidden)

	rr = doJSON(t, h, http.MethodPost, "/backend/servers/create", `{"id":"web 1","address":"example.com"}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/servers/create", `{"id":"web-1","address":"not a host"}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = doJSON(t, h, http.MethodPost, "/backend/servers/create", `{"id":"web-1","address":"example.com"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	if decodeBody(t, rr)["server"].(map[string]any)["address"] != "example.com:22" {
		t.Fatalf("server = %s", rr.Body.String())
	}
	rr = doJSON(t, h, http.MethodPost, "/backend/servers/create", `{"id":"web-1","address":"10.0.0.8:2222"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	if decodeBody(t, rr)["server"].(map[string]any)["address"] != "10.0.0.8:2222" {
		t.Fatalf("updated = %s", rr.Body.String())
	}

	q := dao.Use(app.db)
	if err := q.HostKey.Create(hostKeyRow(t, "web-1")); err != nil {
		t.Fatal(err)
	}
	rr = doJSON(t, h, http.MethodPost, "/backend/servers/delete", `{"id":""}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/servers/delete", `{"id":"web-1"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	if _, err := q.Server.Where(q.Server.ID.Eq("web-1")).First(); err == nil {
		t.Fatal("server was not deleted")
	}
	rows, err := q.HostKey.Where(q.HostKey.ServerID.Eq("web-1")).Find()
	if err != nil || len(rows) != 0 {
		t.Fatalf("host keys left behind: %d %v", len(rows), err)
	}
}

func TestUsersAPI(t *testing.T) {
	app, h := newTestApp(t, Config{})
	insertUser(t, app.db, "alice", "secret1", true, false)
	insertUser(t, app.db, "carol", "secret1", true, false)
	admin := cookieHeader(tokenCookie(t, signIn(t, h, "alice", "secret1")))
	other := tokenCookie(t, signIn(t, h, "carol", "secret1"))

	rr := doJSON(t, h, http.MethodPost, "/backend/users/create", `{"id":"ab","password":"secret1"}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/users/create", `{"id":"dave","password":"short"}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/users/create", `{"id":"dave","password":"secret1","is_admin":true}`, admin)
	requireStatus(t, rr, http.StatusOK)
	created := decodeBody(t, rr)["user"].(map[string]any)
	if created["id"] != "dave" || created["is_admin"] == true || strings.Contains(rr.Body.String(), "$2") {
		t.Fatalf("created = %s", rr.Body.String())
	}

	dave := cookieHeader(tokenCookie(t, signIn(t, h, "dave", "secret1")))
	rr = doJSON(t, h, http.MethodPost, "/backend/users/create", `{"id":"dave","password":"secret2"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	rr = doJSON(t, h, http.MethodGet, "/backend/current_user", "", dave)
	if decodeBody(t, rr)["user"] != nil {
		t.Fatal("password reset should revoke existing sessions")
	}
	requireStatus(t, signIn(t, h, "dave", "secret2"), http.StatusOK)

	rr = doJSON(t, h, http.MethodPost, "/backend/users/update", `{"id":" alice ","is_admin":false}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	if !strings.Contains(rr.Body.String(), "cannot edit self") {
		t.Fatalf("body = %s", rr.Body.String())
	}
	rr = doJSON(t, h, http.MethodPost, "/backend/users/update", `{"id":"missing","is_admin":false}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = doJSON(t, h, http.MethodPost, "/backend/users/update", `{"id":"carol","is_admin":false}`, admin)
	requireStatus(t, rr, http.StatusOK)
	rr = doJSON(t, h, http.MethodGet, "/backend/users", "", cookieHeader(other))
	requireStatus(t, rr, http.StatusForbidden)

	rr = doJSON(t, h, http.MethodPost, "/backend/users/update", `{"id":"dave","is_blocked":true}`, admin)
	requireStatus(t, rr, http.StatusOK)
	rr = signIn(t, h, "dave", "secret2")
	requireStatus(t, rr, http.StatusBadRequest)

	rr = doJSON(t, h, http.MethodGet, "/backend/users", "", admin)
	requireStatus(t, rr, http.StatusOK)
	if strings.Contains(rr.Body.String(), "$2") {
		t.Fatalf("user list leaked a digest: %s", rr.Body.String())
	}
}

func TestGrantsAPI(t *testing.T) {
	app, h := newTestApp(t, Config{})
	insertUser(t, app.db, "alice", "secret1", true, false)
	insertUser(t, app.db, "dave", "secret1", false, false)
	insertServer(t, app.db, "web-1", "example.com:22")
	insertServer(t, app.db, "db-1", "db.internal:22")
	admin := cookieHeader(tokenCookie(t, signIn(t, h, "alice", "secret1")))
	user := cookieHeader(tokenCookie(t, signIn(t, h, "dave", "secret1")))

	rr := doJSON(t, h, http.MethodGet, "/backend/grants", "", admin)
	requireStatus(t, rr, http.StatusOK)
	if grants := decodeBody(t, rr)["grants"].([]any); len(grants) != 0 {
		t.Fatalf("grants = %#v", grants)
	}

	rr = doJSON(t, h, http.MethodPost, "/backend/grants/create", `{"user_id":"missing","server_user":"root","server_id":"web-1"}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/grants/create", `{"user_id":"dave","server_user":"ro ot","server_id":"web-1"}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = doJSON(t, h, http.MethodPost, "/backend/grants/create", `{"user_id":"dave","server_user":"*","server_id":"web*"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	grant := decodeBody(t, rr)["grant"].(map[string]any)
	createdAt, _ := grant["created_at"].(string)
	if createdAt == "" || strings.HasPrefix(createdAt, "0001") {
		t.Fatalf("grant = %#v", grant)
	}
	id := grant["id"].(string)
	rr = doJSON(t, h, http.MethodPost, "/backend/grants/create", `{"user_id":"dave","server_user":"*","server_id":"web*"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	if decodeBody(t, rr)["grant"].(map[string]any)["id"] != id {
		t.Fatalf("duplicate grant = %s", rr.Body.String())
	}

	rr = doJSON(t, h, http.MethodGet, "/backend/grants?user_id=dave", "", admin)
	requireStatus(t, rr, http.StatusOK)
	if grants := decodeBody(t, rr)["grants"].([]any); len(grants) != 1 {
		t.Fatalf("grants = %#v", grants)
	}

	rr = doJSON(t, h, http.MethodGet, "/backend/granted_items", "", user)
	requireStatus(t, rr, http.StatusOK)
	items := decodeBody(t, rr)["granted_items"].([]any)
	if len(items) != 1 {
		t.Fatalf("granted = %#v", items)
	}

	rr = doJSON(t, h, http.MethodPost, "/backend/grants/delete", `{"id":""}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/grants/delete", `{"id":"`+id+`"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	rr = doJSON(t, h, http.MethodGet, "/backend/granted_items", "", user)
	if items = decodeBody(t, rr)["granted_items"].([]any); len(items) != 0 {
		t.Fatalf("granted after delete = %#v", items)
	}
}

func TestAuthorizedKeysAndHostKeysAPI(t *testing.T) {
	app, h := newTestApp(t, Config{})
	insertUser(t, app.db, "alice", "secret1", true, false)
	insertUser(t, app.db, "dave", "secret1", false, false)
	admin := cookieHeader(tokenCookie(t, signIn(t, h, "alice", "secret1")))
	user := cookieHeader(tokenCookie(t, signIn(t, h, "dave", "secret1")))

	rr := doJSON(t, h, http.MethodGet, "/backend/authorized_keys", "", user)
	requireStatus(t, rr, http.StatusForbidden)
	req := doJSON(t, h, http.MethodGet, "/backend/authorized_keys", "", admin)
	requireStatus(t, req, http.StatusOK)
	if req.Body.String() != app.signers.AuthorizedKeys || !strings.Contains(req.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("authorized keys = %q %s", req.Body.String(), req.Header().Get("Content-Type"))
	}

	q := dao.Use(app.db)
	if err := q.HostKey.Create(hostKeyRow(t, "web-1")); err != nil {
		t.Fatal(err)
	}
	rr = doJSON(t, h, http.MethodGet, "/backend/host_keys", "", admin)
	requireStatus(t, rr, http.StatusOK)
	if strings.Contains(rr.Body.String(), "ssh-ed25519 AAAA") {
		t.Fatalf("host key material leaked: %s", rr.Body.String())
	}
	if keys := decodeBody(t, rr)["host_keys"].([]any); len(keys) != 1 {
		t.Fatalf("host keys = %#v", keys)
	}
	rr = doJSON(t, h, http.MethodPost, "/backend/host_keys/delete", `{"server_id":""}`, admin)
	requireStatus(t, rr, http.StatusBadRequest)
	rr = doJSON(t, h, http.MethodPost, "/backend/host_keys/delete", `{"server_id":"web-1"}`, admin)
	requireStatus(t, rr, http.StatusOK)
	rows, err := q.HostKey.Find()
	if err != nil || len(rows) != 0 {
		t.Fatalf("host keys = %d %v", len(rows), err)
	}
}

func httpRequest(remote string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	return req
}

func postSignInFrom(t *testing.T, h http.Handler, user, pass, remote, forwarded string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/backend/sign_in", strings.NewReader(`{"username":"`+user+`","password":"`+pass+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remote
	if forwarded != "" {
		req.Header.Set("X-Forwarded-For", forwarded)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func hostKeyRow(t *testing.T, serverID string) *model.HostKey {
	t.Helper()
	return &model.HostKey{
		ID:          serverID + "-ed25519",
		ServerID:    serverID,
		KeyType:     "ssh-ed25519",
		Fingerprint: "SHA256:test",
		PublicKey:   "ssh-ed25519 AAAA test-host-key\n",
		CreatedAt:   time.Now(),
	}
}
