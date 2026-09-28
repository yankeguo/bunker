package bunker

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yankeguo/bunker/model"
	"github.com/yankeguo/bunker/model/dao"
	"gorm.io/gorm"
)

func TestSQLiteDSNUsesImmediateLock(t *testing.T) {
	dsn := sqliteDSN(filepath.Join("data", "database.sqlite3"))
	for _, part := range []string{"database.sqlite3", "_txlock=immediate", "journal_mode(WAL)", "busy_timeout(10000)"} {
		if !strings.Contains(dsn, part) {
			t.Fatalf("dsn %q missing %s", dsn, part)
		}
	}
}

func TestDatabasePragmas(t *testing.T) {
	db := openTestDB(t)

	var mode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q", mode)
	}

	var timeout int
	if err := db.Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil {
		t.Fatal(err)
	}
	if timeout != 10000 {
		t.Fatalf("busy_timeout = %d", timeout)
	}
}

func TestLegacyTimeStringStillScans(t *testing.T) {
	db := openTestDB(t)
	legacy := time.Date(2024, 8, 5, 12, 0, 0, 0, time.UTC).String()
	err := db.Exec(
		`INSERT INTO users (id, password_digest, created_at, visited_at, is_admin, is_blocked) VALUES (?, ?, ?, ?, 0, 0)`,
		"legacy", "x", legacy, legacy,
	).Error
	if err != nil {
		t.Fatal(err)
	}

	q := dao.Use(db)
	user, err := q.User.Where(q.User.ID.Eq("legacy")).First()
	if err != nil {
		t.Fatal(err)
	}
	if user.CreatedAt.IsZero() || user.CreatedAt.UTC().Format(time.RFC3339) != "2024-08-05T12:00:00Z" {
		t.Fatalf("created_at = %s", user.CreatedAt)
	}
}

func TestCreateDatabaseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	db, err := CreateDatabase(DataDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err = CloseDatabase(db); err != nil {
		t.Fatal(err)
	}
	db, err = CreateDatabase(DataDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err = CloseDatabase(db); err != nil {
		t.Fatal(err)
	}
	if err = CloseDatabase(nil); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeUsers(t *testing.T) {
	dir := t.TempDir()
	db, err := CreateDatabase(DataDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseDatabase(db) })
	log := slog.New(slog.DiscardHandler)

	if err = InitializeUsers(log, DataDir(dir), db); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "users.yaml")
	if err = os.WriteFile(path, []byte(" \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = InitializeUsers(log, DataDir(dir), db); err != nil {
		t.Fatal(err)
	}

	body := "" +
		"username: alice\npassword: secret1\nis_admin: true\n---\n" +
		"username:\npassword: secret1\n---\n" +
		"username: guest\npassword: secret1\n"
	if err = os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = InitializeUsers(log, DataDir(dir), db); err != nil {
		t.Fatal(err)
	}

	q := dao.Use(db)
	alice, err := q.User.Where(q.User.ID.Eq("alice")).First()
	if err != nil || !alice.IsAdmin || !alice.CheckPassword("secret1") {
		t.Fatalf("alice = %+v %v", alice, err)
	}
	if _, err = q.User.Where(q.User.ID.Eq("guest")).First(); err != nil {
		t.Fatal(err)
	}
	users, err := q.User.Find()
	if err != nil || len(users) != 2 {
		t.Fatalf("users = %d %v", len(users), err)
	}

	token := &model.Token{
		ID:        "session-1",
		UserID:    "alice",
		UserAgent: "test",
		CreatedAt: time.Now(),
		VisitedAt: time.Now(),
	}
	if err = q.Token.Create(token); err != nil {
		t.Fatal(err)
	}

	updated := "username: alice\npassword: secret1\nis_admin: false\nupdate_existing: true\n"
	if err = os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = InitializeUsers(log, DataDir(dir), db); err != nil {
		t.Fatal(err)
	}
	alice, err = q.User.Where(q.User.ID.Eq("alice")).First()
	if err != nil || alice.IsAdmin {
		t.Fatalf("alice admin flag = %+v %v", alice, err)
	}
	if _, err = q.Token.Where(q.Token.ID.Eq("session-1")).First(); err != nil {
		t.Fatalf("same password should keep the session: %v", err)
	}

	rotated := "username: alice\npassword: secret2\nis_admin: true\nupdate_existing: true\n"
	if err = os.WriteFile(path, []byte(rotated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = InitializeUsers(log, DataDir(dir), db); err != nil {
		t.Fatal(err)
	}
	alice, err = q.User.Where(q.User.ID.Eq("alice")).First()
	if err != nil || !alice.CheckPassword("secret2") || !alice.IsAdmin {
		t.Fatalf("rotated alice = %+v %v", alice, err)
	}
	if _, err = q.Token.Where(q.Token.ID.Eq("session-1")).First(); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("password change should revoke sessions, err = %v", err)
	}

	kept := "username: guest\npassword: changed\n"
	if err = os.WriteFile(path, []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = InitializeUsers(log, DataDir(dir), db); err != nil {
		t.Fatal(err)
	}
	guest, err := q.User.Where(q.User.ID.Eq("guest")).First()
	if err != nil || !guest.CheckPassword("secret1") {
		t.Fatalf("guest password changed without update_existing: %+v %v", guest, err)
	}
}

func TestInitializeUsersRejectsBadYAML(t *testing.T) {
	dir := t.TempDir()
	db := openTestDB(t)
	path := filepath.Join(dir, "users.yaml")
	if err := os.WriteFile(path, []byte("username: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InitializeUsers(slog.New(slog.DiscardHandler), DataDir(dir), db); err == nil {
		t.Fatal("expected malformed users.yaml to fail")
	}
}
