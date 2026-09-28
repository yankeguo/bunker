package bunker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestLoadConfigTrimsAndTrustsProxy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte("ui:\n  ssh_host: \" bunker.example \"\n  ssh_port: 8022\nserver:\n  listen: \" :9090 \"\n  trust_proxy: true\nssh_server:\n  listen: \":2222\"\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.SSHHost != "bunker.example" || cfg.UI.SSHPort != "8022" {
		t.Fatalf("ui = %+v", cfg.UI)
	}
	if cfg.Server.Listen != ":9090" || !cfg.Server.TrustProxy || cfg.SSHServer.Listen != ":2222" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadConfigAcceptsQuotedPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte("ui:\n  ssh_port: \"8022\"\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.SSHPort != "8022" {
		t.Fatalf("ssh_port = %q", cfg.UI.SSHPort)
	}
}

func TestLoadConfigEmptyFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Listen != ":8080" || cfg.SSHServer.Listen != ":8022" {
		t.Fatalf("listens = %s %s", cfg.Server.Listen, cfg.SSHServer.Listen)
	}
}

func TestLoadConfigRejectsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.yaml")
	if _, err := loadConfigFile(missing); err == nil {
		t.Fatal("expected a missing file to fail")
	}

	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("ui: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfigFile(bad); err == nil {
		t.Fatal("expected malformed yaml to fail")
	}

	port := filepath.Join(dir, "port.yaml")
	if err := os.WriteFile(port, []byte("ui:\n  ssh_port:\n    no: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfigFile(port); err == nil || !strings.Contains(err.Error(), "string") {
		t.Fatalf("port error = %v", err)
	}

	cfg, err := LoadConfig(DataDir(dir))
	if err == nil {
		t.Fatalf("LoadConfig without config.yaml returned %+v", cfg)
	}
}
