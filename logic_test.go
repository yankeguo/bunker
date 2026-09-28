package bunker

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yankeguo/bunker/model"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func TestNormalizeServerAddress(t *testing.T) {
	cases := []struct {
		in, out string
		ok      bool
	}{
		{in: "example.com", out: "example.com:22", ok: true},
		{in: "example.com:2222", out: "example.com:2222", ok: true},
		{in: "127.0.0.1", out: "127.0.0.1:22", ok: true},
		{in: "::1", out: "[::1]:22", ok: true},
		{in: "[::1]:2200", out: "[::1]:2200", ok: true},
		{in: "bad host", ok: false},
		{in: "example.com:99999", ok: false},
		{in: "", ok: false},
		{in: "http://example.com", ok: false},
	}

	for _, tc := range cases {
		got, err := normalizeServerAddress(tc.in)
		if tc.ok && (err != nil || got != tc.out) {
			t.Fatalf("normalizeServerAddress(%q) = %q, %v; want %q", tc.in, got, err, tc.out)
		}
		if !tc.ok && err == nil {
			t.Fatalf("normalizeServerAddress(%q) = %q, want error", tc.in, got)
		}
	}
}

func TestParseSSHTarget(t *testing.T) {
	user, id, err := parseSSHTarget("root@web-1")
	if err != nil || user != "root" || id != "web-1" {
		t.Fatalf("parseSSHTarget(root@web-1) = %q %q %v", user, id, err)
	}
	if _, _, err = parseSSHTarget("root"); err == nil {
		t.Fatal("expected error for a username without a server id")
	}
	if _, _, err = parseSSHTarget("root@web@1"); err == nil {
		t.Fatal("expected error when server id contains @")
	}
	if _, _, err = parseSSHTarget("@web"); err == nil {
		t.Fatal("expected error for an empty server user")
	}
}

func TestExpandGrantedItems(t *testing.T) {
	grants := []*model.Grant{
		{ServerUser: "*", ServerID: "web*"},
		{ServerUser: "deploy", ServerID: "web-1"},
		{ServerUser: "root", ServerID: "db"},
	}
	servers := []*model.Server{
		{ID: "web-1"},
		{ID: "web-2"},
		{ID: "db"},
	}

	items := expandGrantedItems(grants, servers)
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4: %#v", len(items), items)
	}

	want := []grantedItem{
		{ServerUser: "root", ServerID: "db"},
		{ServerUser: "*", ServerID: "web-1"},
		{ServerUser: "deploy", ServerID: "web-1"},
		{ServerUser: "*", ServerID: "web-2"},
	}
	for i, item := range want {
		if items[i] != item {
			t.Fatalf("item %d = %#v, want %#v", i, items[i], item)
		}
	}
}

func TestGrantMatches(t *testing.T) {
	grant := &model.Grant{ServerUser: "web*", ServerID: "prod-*"}
	if !grantMatches(grant, "webapp", "prod-1") {
		t.Fatal("expected wildcard grant to match")
	}
	if grantMatches(grant, "root", "prod-1") {
		t.Fatal("server user wildcard should not match root")
	}
}

func TestParseUserPublicKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))

	key, err := parseUserPublicKey(line + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if key.Type() != ssh.KeyAlgoED25519 {
		t.Fatalf("type = %s", key.Type())
	}

	if _, err = parseUserPublicKey(line + "\n" + line); err == nil {
		t.Fatal("expected two keys to be rejected")
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	rsaSigner, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	small := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(rsaSigner.PublicKey())))
	if _, err = parseUserPublicKey(small); err == nil {
		t.Fatal("expected a 1024-bit RSA key to be rejected")
	}
}

func TestClassifyHostKey(t *testing.T) {
	first := testPublicKey(t)
	second := testPublicKey(t)

	if got := classifyHostKey(nil, first); got != hostKeyRecord {
		t.Fatalf("empty known set = %s", got)
	}

	known := []knownHostKey{{KeyType: first.Type(), Raw: first.Marshal()}}
	if got := classifyHostKey(known, first); got != hostKeyAccept {
		t.Fatalf("same key = %s", got)
	}
	if got := classifyHostKey(known, second); got != hostKeyReject {
		t.Fatalf("changed key = %s", got)
	}
}

func TestVerifyHostKeyPinsAndRejectsChange(t *testing.T) {
	db := openTestDB(t)
	server := &SSHServer{db: db, log: zap.NewNop().Sugar()}

	first := testPublicKey(t)
	second := testPublicKey(t)

	if err := server.verifyHostKey("web-1", first); err != nil {
		t.Fatal(err)
	}
	if err := server.verifyHostKey("web-1", first); err != nil {
		t.Fatal(err)
	}
	if err := server.verifyHostKey("web-1", second); err == nil {
		t.Fatal("expected a changed host key to be rejected")
	}
	if err := server.verifyHostKey("other", second); err != nil {
		t.Fatalf("a different server should pin its own key: %v", err)
	}
}

func TestHTTPServerDoesNotExposePprof(t *testing.T) {
	app := &App{
		log:             zap.NewNop().Sugar(),
		signInLimiter:   newRateLimiter(signInRateLimit, signInRateWindow),
		passwordLimiter: newRateLimiter(passwordRateLimit, passwordRateWindow),
	}
	h := newHandler(app)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("pprof status = %d, want 404", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "profile") {
		t.Fatalf("pprof body leaked: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/metrics", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("metrics status = %d, want 404", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/alive", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "OK" {
		t.Fatalf("liveness = %d %q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing frame protection header")
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/ready", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "OK" {
		t.Fatalf("readiness = %d %q", rr.Code, rr.Body.String())
	}

	form := httptest.NewRequest(http.MethodPost, "/backend/sign_in", strings.NewReader("username=a&password=b"))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, form)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("form sign-in status = %d, body %s", rr.Code, rr.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/backend/sign_in", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, get)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET sign-in status = %d, want 405", rr.Code)
	}

	empty := httptest.NewRequest(http.MethodPost, "/backend/sign_in", strings.NewReader(`{}`))
	empty.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, empty)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "username and password are required") {
		t.Fatalf("empty sign-in = %d %s", rr.Code, rr.Body.String())
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("ui:\n  ssh_host: bunker.example\n  ssh_port: 8022\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.SSHHost != "bunker.example" || cfg.UI.SSHPort != "8022" {
		t.Fatalf("ui = %+v", cfg.UI)
	}
	if cfg.Server.Listen != ":8080" || cfg.SSHServer.Listen != ":8022" {
		t.Fatalf("listens = %s %s", cfg.Server.Listen, cfg.SSHServer.Listen)
	}
	if cfg.Server.TrustProxy {
		t.Fatal("trust_proxy should default to false")
	}
}

func TestRateLimiterWindow(t *testing.T) {
	limiter := newRateLimiter(2, time.Minute)
	if ok, _ := limiter.Allowed("a"); !ok {
		t.Fatal("first attempt should be allowed")
	}
	limiter.Fail("a")
	limiter.Fail("a")
	if ok, wait := limiter.Allowed("a"); ok || wait <= 0 {
		t.Fatalf("limiter allowed=%v wait=%s", ok, wait)
	}
	limiter.Reset("a")
	if ok, _ := limiter.Allowed("a"); !ok {
		t.Fatal("reset should allow the key again")
	}
}

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "db.sqlite3") + "?_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(model.All...); err != nil {
		t.Fatal(err)
	}
	return db
}
