package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndValidation(t *testing.T) {
	t.Setenv("ATRIA2API_KEYS", "")
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error without any upstream key")
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
