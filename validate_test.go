package bunker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
)

func TestNormalizeServerAddress(t *testing.T) {
	cases := []struct {
		in, out string
		ok      bool
	}{
		{in: "example.com", out: "example.com:22", ok: true},
		{in: "  Example.COM:2222 ", out: "Example.COM:2222", ok: true},
		{in: "example.com:022", out: "example.com:22", ok: true},
		{in: "example.com:65535", out: "example.com:65535", ok: true},
		{in: "127.0.0.1", out: "127.0.0.1:22", ok: true},
		{in: "::1", out: "[::1]:22", ok: true},
		{in: "[::1]", out: "[::1]:22", ok: true},
		{in: "[::1]:2200", out: "[::1]:2200", ok: true},
		{in: "my-host.example", out: "my-host.example:22", ok: true},
		{in: "bad host", ok: false},
		{in: "example.com:99999", ok: false},
		{in: "example.com:0", ok: false},
		{in: "", ok: false},
		{in: "http://example.com", ok: false},
		{in: "user@host", ok: false},
		{in: "-bad.example", ok: false},
		{in: "bad-.example", ok: false},
		{in: "example.com.", ok: false},
		{in: "[]", ok: false},
		{in: "[::1]:99999", ok: false},
		{in: "under_score", ok: false},
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
	for _, in := range []string{"root", "root@web@1", "@web", "root@", " root@web", "root@web ", ""} {
		if _, _, err = parseSSHTarget(in); err == nil {
			t.Fatalf("parseSSHTarget(%q) succeeded", in)
		}
	}
}

func TestValidateServerID(t *testing.T) {
	if err := validateServerID("web-1"); err != nil {
		t.Fatal(err)
	}
	if err := validateServerID(strings.Repeat("a", maxServerIDLen)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", strings.Repeat("a", maxServerIDLen+1), "web 1", "a@b", "a/b", "a*b", "a?b", "a[b"} {
		if err := validateServerID(id); err == nil {
			t.Fatalf("validateServerID(%q) succeeded", id)
		}
	}
}

func TestValidateGrantPattern(t *testing.T) {
	if err := validateGrantPattern("server_user", "web*"); err != nil {
		t.Fatal(err)
	}
	if err := validateGrantPattern("server_id", "prod-?"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "a b", "a@b", "a/b", strings.Repeat("a", maxGrantPattern+1)} {
		if err := validateGrantPattern("server_user", value); err == nil {
			t.Fatalf("validateGrantPattern(%q) succeeded", value)
		}
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	name, err := normalizeDisplayName("  laptop ")
	if err != nil || name != "laptop" {
		t.Fatalf("name = %q %v", name, err)
	}
	name, err = normalizeDisplayName("  ")
	if err != nil || name != "Unnamed" {
		t.Fatalf("blank = %q %v", name, err)
	}
	if _, err = normalizeDisplayName("a\nb"); err == nil {
		t.Fatal("expected a line break to be rejected")
	}
	if _, err = normalizeDisplayName(strings.Repeat("名", maxDisplayNameLen+1)); err == nil {
		t.Fatal("expected a long display name to be rejected")
	}
	if _, err = normalizeDisplayName(strings.Repeat("名", maxDisplayNameLen)); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeUserAgent(t *testing.T) {
	if got := normalizeUserAgent("  curl/8\r\nX  "); got != "curl/8  X" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("a", maxUserAgentLen+10)
	if got := normalizeUserAgent(long); got != strings.Repeat("a", maxUserAgentLen) {
		t.Fatalf("len = %d", len(got))
	}

	ua := strings.Repeat("a", maxUserAgentLen-1) + "你"
	got := normalizeUserAgent(ua)
	if got != strings.Repeat("a", maxUserAgentLen-1) || !utf8.ValidString(got) {
		t.Fatalf("truncated %q valid=%v", got, utf8.ValidString(got))
	}
}

func TestParseUserPublicKey(t *testing.T) {
	line := authorizedLine(testPublicKey(t))

	key, err := parseUserPublicKey("# comment\n\n" + line + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if key.Type() != ssh.KeyAlgoED25519 {
		t.Fatalf("type = %s", key.Type())
	}

	if _, err = parseUserPublicKey(""); err == nil {
		t.Fatal("expected an empty key to be rejected")
	}
	if _, err = parseUserPublicKey(line + "\n" + line); err == nil {
		t.Fatal("expected two keys to be rejected")
	}
	if _, err = parseUserPublicKey(strings.Repeat("a", maxPublicKeyLen+1)); err == nil {
		t.Fatal("expected an oversized key to be rejected")
	}
	if _, err = parseUserPublicKey("not a key"); err == nil {
		t.Fatal("expected a malformed key to be rejected")
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	rsaSigner, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = parseUserPublicKey(authorizedLine(rsaSigner.PublicKey())); err == nil {
		t.Fatal("expected a 1024-bit RSA key to be rejected")
	}

	rsaKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaSigner, err = ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseUserPublicKey(authorizedLine(rsaSigner.PublicKey()))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Type() != ssh.KeyAlgoRSA {
		t.Fatalf("rsa type = %s", parsed.Type())
	}

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecSigner, err := ssh.NewSignerFromKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = parseUserPublicKey(authorizedLine(ecSigner.PublicKey())); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePublicKeyRejectsUnknownTypes(t *testing.T) {
	if err := validatePublicKey(stubPublicKey{algo: ssh.KeyAlgoDSA}); err == nil {
		t.Fatal("expected ssh-dss to be rejected")
	}
	if err := validatePublicKey(stubPublicKey{algo: ssh.KeyAlgoRSA}); err == nil {
		t.Fatal("expected an RSA key without a crypto public key to be rejected")
	}
}

type stubPublicKey struct {
	algo string
}

func (k stubPublicKey) Type() string { return k.algo }

func (k stubPublicKey) Marshal() []byte { return []byte("stub") }

func (k stubPublicKey) Verify([]byte, *ssh.Signature) error { return nil }
