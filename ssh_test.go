package bunker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/yankeguo/bunker/model/dao"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func TestWithDefaultSSHPort(t *testing.T) {
	cases := map[string]string{
		"example.com":      "example.com:22",
		"example.com:2222": "example.com:2222",
		"::1":              "[::1]:22",
		"[::1]:2200":       "[::1]:2200",
	}
	for in, want := range cases {
		if got := withDefaultSSHPort(in); got != want {
			t.Fatalf("withDefaultSSHPort(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPublicKeyCallback(t *testing.T) {
	db := openTestDB(t)
	userKey := testSigner(t)
	insertUser(t, db, "alice", "secret1", false, false)
	insertKey(t, db, "alice", "laptop", userKey.PublicKey())
	insertServer(t, db, "web-1", "example.com:22")
	insertGrant(t, db, "alice", "*", "web*")

	server := &SSHServer{db: db, log: slog.New(slog.DiscardHandler)}
	meta := fakeConnMeta{user: "root@web-1", addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1}}

	perm, err := server.PublicKeyCallback(meta, userKey.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if perm.Extensions[sshExtKeyUserID] != "alice" ||
		perm.Extensions[sshExtKeyServerUser] != "root" ||
		perm.Extensions[sshExtKeyServerID] != "web-1" ||
		perm.Extensions[sshExtKeyServerAddress] != "example.com:22" {
		t.Fatalf("extensions = %#v", perm.Extensions)
	}

	for _, user := range []string{"root", "root@web@1", "@web-1"} {
		if _, err = server.PublicKeyCallback(fakeConnMeta{user: user, addr: meta.addr}, userKey.PublicKey()); err == nil {
			t.Fatalf("user %q was accepted", user)
		}
	}
	if _, err = server.PublicKeyCallback(fakeConnMeta{user: "root@missing", addr: meta.addr}, userKey.PublicKey()); err == nil {
		t.Fatal("expected an unknown server to be rejected")
	}
	insertServer(t, db, "db-1", "db.internal:22")
	if _, err = server.PublicKeyCallback(fakeConnMeta{user: "root@db-1", addr: meta.addr}, userKey.PublicKey()); err == nil {
		t.Fatal("expected a missing grant to be rejected")
	}
	if _, err = server.PublicKeyCallback(meta, testPublicKey(t)); err == nil {
		t.Fatal("expected an unknown key to be rejected")
	}

	blockedKey := testSigner(t)
	insertUser(t, db, "bob", "secret1", false, true)
	insertKey(t, db, "bob", "laptop", blockedKey.PublicKey())
	if _, err = server.PublicKeyCallback(meta, blockedKey.PublicKey()); err == nil {
		t.Fatal("expected a blocked user to be rejected")
	}

	orphan := testSigner(t)
	insertKey(t, db, "ghost", "laptop", orphan.PublicKey())
	if _, err = server.PublicKeyCallback(meta, orphan.PublicKey()); err == nil {
		t.Fatal("expected a key with no user to be rejected")
	}

	banner := server.BannerCallback(meta)
	if banner == "" || !strings.Contains(banner, "root@web-1") || !strings.Contains(banner, "127.0.0.1") {
		t.Fatalf("banner = %q", banner)
	}
	server.AuthLogCallback(meta, "publickey", nil)
	server.AuthLogCallback(meta, "publickey", errors.New("nope"))
}

func TestSSHServerListenAndShutdown(t *testing.T) {
	db := openTestDB(t)
	server := NewSSHServer(Config{}, db, &Signers{Host: []ssh.Signer{testSigner(t)}}, slog.New(slog.DiscardHandler))
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	server.listen = "127.0.0.1:bad"
	if err := server.ListenAndServe(); err == nil {
		t.Fatal("expected an invalid listen address to fail")
	}

	server = NewSSHServer(Config{}, db, &Signers{Host: []ssh.Signer{testSigner(t)}}, slog.New(slog.DiscardHandler))
	server.listen = "127.0.0.1:0"
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	addr := waitSSHListener(t, server)

	if err := server.ListenAndServe(); err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("second listen = %v", err)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	if err = server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ssh server did not stop")
	}
}

func TestSSHSessionProxyPinsHostKey(t *testing.T) {
	db := openTestDB(t)
	userKey := testSigner(t)
	clientKey := testSigner(t)
	hostKey := testSigner(t)

	insertUser(t, db, "alice", "secret1", false, false)
	insertKey(t, db, "alice", "laptop", userKey.PublicKey())
	insertGrant(t, db, "alice", "root", "web-1")

	targetAddr, _ := startTargetSSH(t, clientKey.PublicKey())
	insertServer(t, db, "web-1", targetAddr)

	bunker := startBunkerSSH(t, db, hostKey, clientKey)
	client := dialBunker(t, bunker, userKey, "root@web-1")
	out, err := runBunkerCommand(client)
	_ = client.Close()
	if err != nil || out != "pong" {
		t.Fatalf("first session = %q %v", out, err)
	}

	client = dialBunker(t, bunker, userKey, "root@web-1")
	out, err = runBunkerCommand(client)
	_ = client.Close()
	if err != nil || out != "pong" {
		t.Fatalf("second session = %q %v", out, err)
	}

	replacement, _ := startTargetSSH(t, clientKey.PublicKey())
	q := dao.Use(db)
	if _, err = q.Server.Where(q.Server.ID.Eq("web-1")).UpdateColumnSimple(q.Server.Address.Value(replacement)); err != nil {
		t.Fatal(err)
	}
	client = dialBunker(t, bunker, userKey, "root@web-1")
	_, err = runBunkerCommand(client)
	_ = client.Close()
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed host key error = %v", err)
	}

	if _, err = q.HostKey.Where(q.HostKey.ServerID.Eq("web-1")).Delete(); err != nil {
		t.Fatal(err)
	}
	client = dialBunker(t, bunker, userKey, "root@web-1")
	defer client.Close()
	out, err = runBunkerCommand(client)
	if err != nil || out != "pong" {
		t.Fatalf("session after host key reset = %q %v", out, err)
	}
}

type fakeConnMeta struct {
	user string
	addr net.Addr
}

func (f fakeConnMeta) User() string          { return f.user }
func (f fakeConnMeta) SessionID() []byte     { return []byte("session-id-value") }
func (f fakeConnMeta) ClientVersion() []byte { return []byte("SSH-2.0-test") }
func (f fakeConnMeta) ServerVersion() []byte { return []byte("SSH-2.0-bunker") }
func (f fakeConnMeta) RemoteAddr() net.Addr  { return f.addr }
func (f fakeConnMeta) LocalAddr() net.Addr   { return f.addr }

func startTargetSSH(t *testing.T, clientKey ssh.PublicKey) (string, ssh.Signer) {
	t.Helper()
	host := testSigner(t)
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() == "root" && bytes.Equal(key.Marshal(), clientKey.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("denied")
		},
	}
	cfg.AddHostKey(host)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveTargetSession(conn, cfg)
		}
	}()
	return ln.Addr().String(), host
}

func serveTargetSession(conn net.Conn, cfg *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range requests {
				if req.Type == "exec" || req.Type == "shell" {
					if req.WantReply {
						_ = req.Reply(true, nil)
					}
					_, _ = io.WriteString(channel, "pong")
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct {
						Status uint32
					}{Status: 0}))
					return
				}
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
			}
		}()
	}
}

func startBunkerSSH(t *testing.T, db *gorm.DB, hostKey, clientKey ssh.Signer) string {
	t.Helper()
	server := NewSSHServer(Config{}, db, &Signers{
		Host:   []ssh.Signer{hostKey},
		Client: []ssh.Signer{clientKey},
	}, slog.New(slog.DiscardHandler))
	server.listen = "127.0.0.1:0"
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
	})
	return waitSSHListener(t, server)
}

func waitSSHListener(t *testing.T, server *SSHServer) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		server.mu.Lock()
		ln := server.listener
		server.mu.Unlock()
		if ln != nil {
			return ln.Addr().String()
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("ssh server did not start listening")
	return ""
}

func dialBunker(t *testing.T, addr string, signer ssh.Signer, user string) *ssh.Client {
	t.Helper()
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return nil
		},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial %s as %s: %v", addr, user, err)
	}
	return client
}

func runBunkerCommand(client *ssh.Client) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	type result struct {
		out []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := sess.Output("true")
		ch <- result{out, err}
	}()
	select {
	case res := <-ch:
		return string(res.out), res.err
	case <-time.After(5 * time.Second):
		return "", errors.New("session timed out")
	}
}
