package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `listen = "127.0.0.1:9000"
username = "user"
password = "pass"
source_ips = ["2001:db8::1"]
rotation = "round_robin"
allow_ports = [443]

[timeouts]
connect = "2s"
read_header = "3s"
idle = "4s"
request = "5s"
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:9000" || cfg.Rotation != "round_robin" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Timeouts.Connect.String() != "2s" || cfg.Timeouts.Request.String() != "5s" {
		t.Fatalf("unexpected timeout values: %+v", cfg.Timeouts)
	}
	if cfg.BlockPrivate == nil || !*cfg.BlockPrivate {
		t.Fatal("private destination blocking should default to true")
	}
}

func TestLoadRequiresCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `source_ips = ["2001:db8::1"]
username = "user"
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected missing password to be rejected")
	}
}

func TestLoadAllowsDisabledAuthentication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `auth_enabled = false
source_ips = ["2001:db8::1"]
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthEnabled == nil || *cfg.AuthEnabled {
		t.Fatal("authentication should be disabled")
	}
}
