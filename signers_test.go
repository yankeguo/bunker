package bunker

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateSigner(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	path := filepath.Join(dir, "ssh_host_ed25519_key")

	first, err := loadOrCreateSigner(log, path, sshPrivateKeyGenerators[0].generate)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}

	second, err := loadOrCreateSigner(log, path, sshPrivateKeyGenerators[0].generate)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.PublicKey().Marshal(), second.PublicKey().Marshal()) {
		t.Fatal("reloading the signer changed the public key")
	}

	if err = os.WriteFile(path, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = loadOrCreateSigner(log, path, sshPrivateKeyGenerators[0].generate); err == nil {
		t.Fatal("expected a corrupt key file to fail")
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "not a key" {
		t.Fatal("corrupt key file was overwritten")
	}

	missing := filepath.Join(dir, "missing-dir", "ssh_host_ed25519_key")
	if _, err = loadOrCreateSigner(log, missing, sshPrivateKeyGenerators[0].generate); err == nil {
		t.Fatal("expected a missing parent directory to fail")
	}
	if _, err = os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("stat missing key = %v", err)
	}
}

func TestCreateSigners(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	signers, err := CreateSigners(log, DataDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(signers.Host) != 3 || len(signers.Client) != 3 {
		t.Fatalf("host=%d client=%d", len(signers.Host), len(signers.Client))
	}
	if !strings.Contains(signers.AuthorizedKeys, "ssh-ed25519") {
		t.Fatalf("authorized keys = %s", signers.AuthorizedKeys)
	}
	for _, name := range []string{
		"ssh_host_ed25519_key", "ssh_host_ecdsa_key", "ssh_host_rsa_key",
		"ssh_client_ed25519_key", "ssh_client_ecdsa_key", "ssh_client_rsa_key",
	} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o", name, info.Mode().Perm())
		}
	}

	again, err := CreateSigners(log, DataDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if again.AuthorizedKeys != signers.AuthorizedKeys {
		t.Fatal("recreating signers changed the client public keys")
	}
}
