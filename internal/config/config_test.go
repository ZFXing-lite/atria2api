package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndValidation(t *testing.T) {
	t.Setenv("ATRIA2API_KEYS", "")
	empty, err := Load("")
	if err != nil {
		t.Fatalf("empty key list must boot so the panel can add the first key: %v", err)
	}
	if len(empty.Upstream.Keys) != 0 {
		t.Fatalf("expected no keys, got %d", len(empty.Upstream.Keys))
	}

	t.Setenv("ATRIA2API_KEYS", "atr_a, atr_b")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Upstream.BaseURL != DefaultBaseURL {
		t.Fatalf("base url = %s", cfg.Upstream.BaseURL)
	}
	if len(cfg.Upstream.Keys) != 2 {
		t.Fatalf("keys = %d", len(cfg.Upstream.Keys))
	}
	if cfg.ListenAddr() != ":8318" {
		t.Fatalf("listen = %s", cfg.ListenAddr())
	}
	if !cfg.Upstream.ForceModel {
		t.Fatal("force-model should default to true")
	}
}

func TestYamlLoadAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "port: 9999\napi-keys:\n  - gw-x\nupstream:\n  keys:\n    - key: atr_yaml\n    - key: atr_yaml\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// env must be cleared or the duplicate-key validation below is masked
	t.Setenv("ATRIA2API_KEYS", "")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected duplicate-key error")
	}

	if err := os.WriteFile(path, []byte("port: 9999\napi-keys:\n  - gw-x\nupstream:\n  base-url: https://example.com/\n  keys:\n    - key: atr_yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Port != 9999 || cfg.Upstream.BaseURL != "https://example.com" {
		t.Fatalf("yaml not applied: port=%d base=%s", cfg.Port, cfg.Upstream.BaseURL)
	}
	if !cfg.IsAuthorized("gw-x") || cfg.IsAuthorized("gw-y") {
		t.Fatalf("api-key auth mismatch")
	}

	t.Setenv("ATRIA2API_PORT", "7777")
	t.Setenv("ATRIA2API_BASE_URL", "https://env.example/")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("load with env: %v", err)
	}
	if cfg.Port != 7777 {
		t.Fatalf("env port override failed: %d", cfg.Port)
	}
	if cfg.Upstream.BaseURL != "https://env.example" {
		t.Fatalf("env base url override failed: %s", cfg.Upstream.BaseURL)
	}
}

func TestSnapshotMasksKeys(t *testing.T) {
	cfg := Default()
	cfg.Upstream.Keys = []UpstreamKey{{Key: "atr_supersecretkey", Weight: 1}}
	cfg.APIKeys = []string{"gw-secret"}
	b, err := cfg.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if contains(s, "atr_supersecretkey") {
		t.Fatalf("upstream key leaked into snapshot: %s", s)
	}
	if contains(s, "gw-secret") {
		t.Fatalf("gateway key leaked into snapshot: %s", s)
	}
	if !contains(s, "atr_") {
		t.Fatalf("masked key prefix missing: %s", s)
	}
}

func TestMaskKey(t *testing.T) {
	if m := MaskKey("atr_abcdefghij"); m == "atr_abcdefghij" || len(m) < 6 {
		t.Fatalf("masking too weak: %s", m)
	}
	if m := MaskKey("short"); m != "*****" {
		t.Fatalf("short key should be fully masked: %s", m)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return len(needle) == 0
}

func TestEnvAllowRemoteAndMgmtKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	os.WriteFile(p, []byte("host: 127.0.0.1\nport: 8318\nupstream:\n  keys:\n    - key: atr_t\n"), 0o600)

	t.Run("off by default", func(t *testing.T) {
		c, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		if c.Management.AllowRemote {
			t.Fatal("allow-remote must default to false")
		}
	})
	t.Run("env enables remote and sets key", func(t *testing.T) {
		t.Setenv("ATRIA2API_ALLOW_REMOTE", "1")
		t.Setenv("ATRIA2API_MGMT_KEY", "panel-pass")
		c, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		if !c.Management.AllowRemote {
			t.Fatal("ATRIA2API_ALLOW_REMOTE=1 must enable remote")
		}
		if c.Management.SecretKey != "panel-pass" {
			t.Fatalf("mgmt key not applied: %q", c.Management.SecretKey)
		}
	})
	t.Run("env truthy spellings", func(t *testing.T) {
		for _, v := range []string{"true", "TRUE", "yes", "1"} {
			t.Setenv("ATRIA2API_ALLOW_REMOTE", v)
			c, err := Load(p)
			if err != nil {
				t.Fatal(err)
			}
			if !c.Management.AllowRemote {
				t.Fatalf("%q must be treated as true", v)
			}
		}
		t.Setenv("ATRIA2API_ALLOW_REMOTE", "0")
		c, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		if c.Management.AllowRemote {
			t.Fatal("\"0\" must not enable remote")
		}
	})
}
