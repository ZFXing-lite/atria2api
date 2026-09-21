package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ZFXing-lite/atria2api/internal/config"
	"github.com/ZFXing-lite/atria2api/internal/keypool"
	"github.com/ZFXing-lite/atria2api/internal/metrics"
	"github.com/ZFXing-lite/atria2api/internal/relay"
)

func bulkServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	pool := keypool.New([]keypool.UpstreamKey{{Key: "atr_old"}}, keypool.Settings{}, "")
	r := relay.New(pool, nil, relay.Config{BaseURL: "http://up.invalid",
		DefaultModel: "Atria-Dawn-Preview", ForceModel: true}, nil)
	cfg := &config.Config{Host: "127.0.0.1", Port: 8318, APIKeys: []string{"gw"}}
	cfg.Upstream.BaseURL = "http://up.invalid"
	cfg.Upstream.DefaultModel = "Atria-Dawn-Preview"
	cfg.Management.SecretKey = "mgt"
	cfg.Management.AllowRemote = true
	return New(cfg, r, pool, nil, metrics.New(""), cfgPath), cfgPath
}

func TestBulkImport(t *testing.T) {
	s, cfgPath := bulkServer(t)

	req := httptest.NewRequest("POST", "/v0/management/keys/bulk",
		strings.NewReader(`{"keys":["atr_a","atr_b","","  atr_a  ","# comment","atr_c"],"weight":2}`))
	req.Header.Set("X-Management-Key", "mgt")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("bulk: %d %s", w.Code, w.Body.String())
	}
	var res struct {
		Added   int      `json:"added"`
		Updated int      `json:"updated"`
		Skipped int      `json:"skipped"`
		IDs     []string `json:"ids"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	// 3 new (a,b,c); ignored: blank line, in-batch duplicate, comment line.
	if res.Added != 3 || res.Updated != 0 || res.Skipped != 3 {
		t.Fatalf("counts wrong: +%d ~%d /%d (want 3/0/3)", res.Added, res.Updated, res.Skipped)
	}
	if len(res.IDs) != 3 {
		t.Fatalf("ids = %v", res.IDs)
	}

	total, healthy := s.pool.Summary()
	if total != 4 || healthy != 4 {
		t.Fatalf("pool should have 4 keys, got %d/%d", total, healthy)
	}
	b, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(b), "atr_a") || !strings.Contains(string(b), "atr_c") {
		t.Fatalf("keys not persisted: %s", b)
	}
}

func TestBulkImportUpdatesExisting(t *testing.T) {
	s, _ := bulkServer(t)

	req := httptest.NewRequest("POST", "/v0/management/keys/bulk",
		strings.NewReader(`{"keys":["atr_old","atr_new"],"weight":5}`))
	req.Header.Set("X-Management-Key", "mgt")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var res struct {
		Added   int `json:"added"`
		Updated int `json:"updated"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Added != 1 || res.Updated != 1 {
		t.Fatalf("want 1 added 1 updated, got +%d ~%d", res.Added, res.Updated)
	}
	total, _ := s.pool.Summary()
	if total != 2 {
		t.Fatalf("pool should have 2 keys, got %d", total)
	}
	// Weight of the existing key must have been refreshed to 5.
	for _, st := range s.pool.Status() {
		if st.Weight != 5 {
			t.Fatalf("weight not applied: %+v", st)
		}
	}
}

func TestBulkImportRejectsEmpty(t *testing.T) {
	s, _ := bulkServer(t)
	req := httptest.NewRequest("POST", "/v0/management/keys/bulk",
		strings.NewReader(`{"keys":["   ","# only a comment"]}`))
	req.Header.Set("X-Management-Key", "mgt")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("bulk: %d", w.Code)
	}
	var res struct {
		Added int `json:"added"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Added != 0 {
		t.Fatalf("expected 0 added, got %d", res.Added)
	}
}

func TestBulkImportNeedsKey(t *testing.T) {
	s, _ := bulkServer(t)
	req := httptest.NewRequest("POST", "/v0/management/keys/bulk",
		strings.NewReader(`{"keys":["atr_a"]}`))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("unauthenticated bulk must be 401, got %d", w.Code)
	}
}
