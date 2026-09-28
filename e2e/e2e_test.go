package e2e

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

var bunkerBin string

func TestMain(m *testing.M) {
	root := moduleRoot()
	dir, err := os.MkdirTemp("", "bunker-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	bunkerBin = filepath.Join(dir, "bunker")
	cmd := exec.Command("go", "build", "-o", bunkerBin, "./cmd/bunker")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build bunker: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestBunkerExitsWhenConfigIsMissing(t *testing.T) {
	cmd := exec.Command(bunkerBin, "--data-dir", t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected a missing config.yaml to fail, output %s", out)
	}
	if !strings.Contains(string(out), "load config") {
		t.Fatalf("output = %s", out)
	}
}

func TestBunkerHTTPAndSSHProxy(t *testing.T) {
	base, sshAddr, logs, stop := startBunker(t)

	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index status = %d\n%s", resp.StatusCode, logs.String())
	}

	ready, err := http.Get(base + "/debug/ready")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(ready.Body)
	_ = ready.Body.Close()
	if ready.StatusCode != http.StatusOK || string(body) != "OK" {
		t.Fatalf("ready = %d %q", ready.StatusCode, body)
	}

	admin := newAPI(t, base)
	admin.call(http.MethodPost, "/backend/sign_in", map[string]string{
		"username": "admin",
		"password": "wrong",
	}, http.StatusBadRequest)
	admin.call(http.MethodPost, "/backend/sign_in", map[string]string{
		"username": "admin",
		"password": "secret-admin",
	}, http.StatusOK)

	opts := admin.call(http.MethodGet, "/backend/ui_options", nil, http.StatusOK)
	if opts["ssh_host"] != "bunker.example" || opts["ssh_port"] != "8022" {
		t.Fatalf("ui options = %#v", opts)
	}
	authorized := admin.text(http.MethodGet, "/backend/authorized_keys", http.StatusOK)
	if !strings.Contains(authorized, "ssh-ed25519") {
		t.Fatalf("authorized keys = %q", authorized)
	}

	targetAddr := startTargetSSH(t, parseAuthorizedKeys(t, authorized))
	admin.call(http.MethodPost, "/backend/servers/create", map[string]string{
		"id":      "web-1",
		"address": targetAddr,
	}, http.StatusOK)
	admin.call(http.MethodPost, "/backend/servers/create", map[string]string{
		"id":      "db-1",
		"address": targetAddr,
	}, http.StatusOK)
	admin.call(http.MethodPost, "/backend/users/create", map[string]string{
		"id":       "alice",
		"password": "secret-user",
	}, http.StatusOK)
	admin.call(http.MethodPost, "/backend/users/create", map[string]string{
		"id":       "bobby",
		"password": "secret-user",
	}, http.StatusOK)
	admin.call(http.MethodPost, "/backend/grants/create", map[string]string{
		"user_id":     "alice",
		"server_user": "root",
		"server_id":   "web-1",
	}, http.StatusOK)

	aliceKey := testSigner(t)
	alice := newAPI(t, base)
	alice.call(http.MethodPost, "/backend/sign_in", map[string]string{
		"username": "alice",
		"password": "secret-user",
	}, http.StatusOK)
	alice.text(http.MethodGet, "/backend/authorized_keys", http.StatusForbidden)
	alice.call(http.MethodPost, "/backend/keys/create", map[string]string{
		"display_name": "laptop",
		"public_key":   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(aliceKey.PublicKey()))),
	}, http.StatusOK)

	bobby := newAPI(t, base)
	bobby.call(http.MethodPost, "/backend/sign_in", map[string]string{
		"username": "bobby",
		"password": "secret-user",
	}, http.StatusOK)
	bobby.call(http.MethodGet, "/backend/servers", nil, http.StatusForbidden)

	out, err := sshExec(sshAddr, "root@web-1", aliceKey)
	if err != nil || out != "pong" {
		t.Fatalf("proxied session = %q %v\n%s", out, err, logs.String())
	}
	out, err = sshExec(sshAddr, "root@web-1", aliceKey)
	if err != nil || out != "pong" {
		t.Fatalf("second session = %q %v\n%s", out, err, logs.String())
	}
	if _, err = sshExec(sshAddr, "root@db-1", aliceKey); err == nil {
		t.Fatal("expected a server without a grant to be rejected")
	}

	replacement := startTargetSSH(t, parseAuthorizedKeys(t, authorized))
	admin.call(http.MethodPost, "/backend/servers/create", map[string]string{
		"id":      "web-1",
		"address": replacement,
	}, http.StatusOK)
	if _, err = sshExec(sshAddr, "root@web-1", aliceKey); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed host key error = %v\n%s", err, logs.String())
	}
	admin.call(http.MethodPost, "/backend/host_keys/delete", map[string]string{
		"server_id": "web-1",
	}, http.StatusOK)
	out, err = sshExec(sshAddr, "root@web-1", aliceKey)
	if err != nil || out != "pong" {
		t.Fatalf("session after host key reset = %q %v\n%s", out, err, logs.String())
	}

	alice.call(http.MethodPost, "/backend/sign_out", map[string]any{}, http.StatusOK)
	alice.call(http.MethodGet, "/backend/keys", nil, http.StatusUnauthorized)

	if err = stop(); err != nil {
		t.Fatalf("shutdown exit = %v\n%s", err, logs.String())
	}
	conn, err := net.DialTimeout("tcp", sshAddr, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("ssh port still accepted a connection after shutdown")
	}
}

func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err = os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found")
		}
		dir = parent
	}
}

type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func startBunker(t *testing.T) (base, sshAddr string, logs *logBuffer, stop func() error) {
	t.Helper()
	httpAddr := reserveAddr(t)
	sshAddr = reserveAddr(t)
	data := t.TempDir()
	config := fmt.Sprintf("ui:\n  ssh_host: bunker.example\n  ssh_port: \"8022\"\nserver:\n  listen: %q\nssh_server:\n  listen: %q\n", httpAddr, sshAddr)
	if err := os.WriteFile(filepath.Join(data, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	users := "username: admin\npassword: secret-admin\nis_admin: true\n"
	if err := os.WriteFile(filepath.Join(data, "users.yaml"), []byte(users), 0o644); err != nil {
		t.Fatal(err)
	}

	logs = &logBuffer{}
	cmd := exec.Command(bunkerBin, "--data-dir", data)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var exitErr error
	go func() {
		exitErr = cmd.Wait()
		close(done)
	}()

	var once sync.Once
	stop = func() error {
		once.Do(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
			select {
			case <-done:
			case <-time.After(12 * time.Second):
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				<-done
			}
		})
		return exitErr
	}
	t.Cleanup(func() { _ = stop() })

	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get("http://" + httpAddr + "/debug/alive")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return "http://" + httpAddr, sshAddr, logs, stop
			}
		}
		select {
		case <-done:
			t.Fatalf("bunker exited before ready: %v\n%s", exitErr, logs.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("bunker did not become ready\n%s", logs.String())
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func reserveAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err = ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func newAPI(t *testing.T, base string) *apiClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &apiClient{
		t:    t,
		base: base,
		http: &http.Client{Jar: jar, Timeout: 15 * time.Second},
	}
}

func (a *apiClient) call(method, path string, body any, want int) map[string]any {
	a.t.Helper()
	raw, resp := a.do(method, path, body)
	if resp.StatusCode != want {
		a.t.Fatalf("%s %s = %d, want %d, body %s", method, path, resp.StatusCode, want, raw)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		a.t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func (a *apiClient) text(method, path string, want int) string {
	a.t.Helper()
	raw, resp := a.do(method, path, nil)
	if resp.StatusCode != want {
		a.t.Fatalf("%s %s = %d, want %d, body %s", method, path, resp.StatusCode, want, raw)
	}
	return string(raw)
}

func (a *apiClient) do(method, path string, body any) ([]byte, *http.Response) {
	a.t.Helper()
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, a.base+path, reader)
	if err != nil {
		a.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		a.t.Fatal(err)
	}
	return raw, resp
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

func parseAuthorizedKeys(t *testing.T, text string) []ssh.PublicKey {
	t.Helper()
	var keys []ssh.PublicKey
	rest := []byte(text)
	for len(bytes.TrimSpace(rest)) > 0 {
		pub, _, _, rem, err := ssh.ParseAuthorizedKey(rest)
		if err != nil {
			t.Fatalf("parse authorized keys: %v\n%s", err, text)
		}
		keys = append(keys, pub)
		rest = rem
	}
	if len(keys) == 0 {
		t.Fatal("no authorized keys")
	}
	return keys
}

func startTargetSSH(t *testing.T, allowed []ssh.PublicKey) string {
	t.Helper()
	host := testSigner(t)
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() != "root" {
				return nil, errors.New("denied")
			}
			for _, item := range allowed {
				if bytes.Equal(item.Marshal(), key.Marshal()) {
					return &ssh.Permissions{}, nil
				}
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
			go serveTarget(conn, cfg)
		}
	}()
	return ln.Addr().String()
}

func serveTarget(conn net.Conn, cfg *ssh.ServerConfig) {
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

func sshExec(addr, user string, signer ssh.Signer) (string, error) {
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return nil
		},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return "", err
	}
	defer client.Close()

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
		out, err := sess.CombinedOutput("true")
		ch <- result{out, err}
	}()
	select {
	case res := <-ch:
		return string(res.out), res.err
	case <-time.After(8 * time.Second):
		return "", errors.New("session timed out")
	}
}
