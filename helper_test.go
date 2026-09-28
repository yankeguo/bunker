package bunker

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	// HTTP tests serve the embedded UI. DEBUG=ui would proxy to localhost:3000
	// instead, and DEBUG=db would dump SQL.
	saved := _debug
	_debug = map[string]bool{}
	code := m.Run()
	_debug = saved
	os.Exit(code)
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := CreateDatabase(DataDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseDatabase(db) })
	return db
}

func testSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	return testSigner(t).PublicKey()
}

func authorizedLine(key ssh.PublicKey) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}

func newTestApp(t *testing.T, cfg Config) (*App, http.Handler) {
	t.Helper()
	db := openTestDB(t)
	app := CreateApp(db, cfg, slog.New(slog.DiscardHandler), &Signers{
		AuthorizedKeys: "ssh-ed25519 AAAA test-key\n",
	})
	return app, newHandler(app)
}

func insertUser(t *testing.T, db *gorm.DB, id, password string, admin, blocked bool) *model.User {
	t.Helper()
	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := &model.User{
		ID:             id,
		PasswordDigest: string(digest),
		CreatedAt:      time.Now(),
		VisitedAt:      time.Now(),
		IsAdmin:        admin,
		IsBlocked:      blocked,
	}
	if err = dao.Use(db).User.Create(user); err != nil {
		t.Fatal(err)
	}
	return user
}

func insertKey(t *testing.T, db *gorm.DB, userID, name string, pub ssh.PublicKey) {
	t.Helper()
	key := &model.Key{
		ID:          ssh.FingerprintSHA256(pub),
		DisplayName: name,
		UserID:      userID,
		CreatedAt:   time.Now(),
	}
	if err := dao.Use(db).Key.Create(key); err != nil {
		t.Fatal(err)
	}
}

func insertServer(t *testing.T, db *gorm.DB, id, address string) {
	t.Helper()
	server := &model.Server{ID: id, Address: address, CreatedAt: time.Now()}
	if err := dao.Use(db).Server.Create(server); err != nil {
		t.Fatal(err)
	}
}

func insertGrant(t *testing.T, db *gorm.DB, userID, serverUser, serverID string) {
	t.Helper()
	grant := &model.Grant{
		ID:         userID + ":" + serverUser + ":" + serverID,
		UserID:     userID,
		ServerUser: serverUser,
		ServerID:   serverID,
		CreatedAt:  time.Now(),
	}
	if err := dao.Use(db).Grant.Create(grant); err != nil {
		t.Fatal(err)
	}
}

func doJSON(t *testing.T, h http.Handler, method, path, body, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" || method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func requireStatus(t *testing.T, rr *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rr.Code != code {
		t.Fatalf("status = %d, want %d, body %s", rr.Code, code, rr.Body.String())
	}
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rr.Body.String(), err)
	}
	return body
}

func tokenCookie(t *testing.T, rr *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	res := rr.Result()
	defer res.Body.Close()
	for _, cookie := range res.Cookies() {
		if cookie.Name == "token" {
			return cookie
		}
	}
	t.Fatalf("missing token cookie, body %s", rr.Body.String())
	return nil
}

func cookieHeader(cookie *http.Cookie) string {
	if cookie == nil || cookie.Value == "" {
		return ""
	}
	return "token=" + cookie.Value
}

func signIn(t *testing.T, h http.Handler, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, h, http.MethodPost, "/backend/sign_in", `{"username":"`+username+`","password":"`+password+`"}`, "")
}
