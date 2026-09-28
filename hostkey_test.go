package bunker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"golang.org/x/crypto/ssh"
)

func TestClassifyHostKey(t *testing.T) {
	first := testPublicKey(t)
	second := testPublicKey(t)
	other := testECDSAPublicKey(t)

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
	if got := classifyHostKey(known, other); got != hostKeyRecord {
		t.Fatalf("new algorithm = %s", got)
	}
}

func TestHostKeyIDIsStable(t *testing.T) {
	if hostKeyID("web-1", ssh.KeyAlgoED25519) != hostKeyID("web-1", ssh.KeyAlgoED25519) {
		t.Fatal("host key id changed")
	}
	if hostKeyID("web-1", ssh.KeyAlgoED25519) == hostKeyID("web-2", ssh.KeyAlgoED25519) {
		t.Fatal("different servers produced the same id")
	}
}

func TestVerifyHostKeyPinsAndRejectsChange(t *testing.T) {
	db := openTestDB(t)
	server := &SSHServer{db: db, log: slog.New(slog.DiscardHandler)}

	first := testPublicKey(t)
	second := testPublicKey(t)
	other := testECDSAPublicKey(t)

	if err := server.verifyHostKey("web-1", first); err != nil {
		t.Fatal(err)
	}
	if err := server.verifyHostKey("web-1", first); err != nil {
		t.Fatal(err)
	}
	if err := server.verifyHostKey("web-1", other); err != nil {
		t.Fatalf("a second algorithm should be recorded: %v", err)
	}
	if err := server.verifyHostKey("web-1", second); err == nil {
		t.Fatal("expected a changed host key to be rejected")
	}
	if err := server.verifyHostKey("other", second); err != nil {
		t.Fatalf("a different server should pin its own key: %v", err)
	}
}

func TestVerifyHostKeyUnreadableRows(t *testing.T) {
	db := openTestDB(t)
	server := &SSHServer{db: db, log: slog.New(slog.DiscardHandler)}
	q := dao.Use(db)

	key := testPublicKey(t)
	if err := q.HostKey.Create(&model.HostKey{
		ID:          hostKeyID("web-1", key.Type()),
		ServerID:    "web-1",
		KeyType:     key.Type(),
		Fingerprint: "bad",
		PublicKey:   "not-a-key",
		CreatedAt:   time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	err := server.verifyHostKey("web-1", key)
	if err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("err = %v", err)
	}

	if err = q.HostKey.Create(&model.HostKey{
		ID:          "other-row",
		ServerID:    "web-2",
		KeyType:     key.Type(),
		Fingerprint: "bad",
		PublicKey:   "not-a-key",
		CreatedAt:   time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err = server.verifyHostKey("web-2", key); err != nil {
		t.Fatalf("unreadable row with a different id should be skipped: %v", err)
	}
}

func testECDSAPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}
