package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZFXing-lite/atria2api/internal/config"
	"github.com/ZFXing-lite/atria2api/internal/keypool"
	"github.com/ZFXing-lite/atria2api/internal/metrics"
	"github.com/ZFXing-lite/atria2api/internal/relay"
)

// mgmtServer builds a server whose management API is open (test origin is not
// loopback) and whose config persists to a temp file.
func mgmtServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"model":"Atria-Dawn-Preview","choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	t.Cleanup(up.Close)

	pool := keypool.New([]keypool.UpstreamKey{{Key: "atr_orig"}}, keypool.Settings{MaxRetries: 2}, "")
	r := relay.New(pool, nil, relay.Config{
		BaseURL: up.URL, DefaultModel: "Atria-Dawn-Preview", ForceModel: true, Timeout: 10_000_000_000,
	}, nil)

	cfg := &config.Config{Host: "127.0.0.1", Port: 8318, APIKeys: []string{"gw-orig", "gw-other"}}
	cfg.Upstream.BaseURL = up.URL
	cfg.Upstream.DefaultModel = "Atria-Dawn-Preview"
	cfg.Management.SecretKey = "mgt"
	cfg.Management.AllowRemote = true
	s := New(cfg, r, pool, nil, metrics.New(""), cfgPath)
	return s, cfgPath
}

func mgmtDo(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = strings.NewReader(string(b))
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("X-Management-Key", "mgt")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func TestPanelServed(t *testing.T) {
	s, _ := mgmtServer(t)
	w := mgmtDo(t, s, "GET", "/v0/management/panel", nil)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "控制面板") {
		t.Fatalf("panel not served: %d %s", w.Code, firstChars(body))
	}
}

func TestUpstreamKeyAddRemove(t *testing.T) {
	s, cfgPath := mgmtServer(t)

	w := mgmtDo(t, s, "POST", "/v0/management/keys", map[string]any{"key": "atr_new", "weight": 3})
	if w.Code != 200 {
		t.Fatalf("add: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ID == "" {
		t.Fatal("no id returned")
	}

	// The pool must serve with the new key immediately.
	total, healthy := s.pool.Summary()
	if total != 2 || healthy != 2 {
		t.Fatalf("pool not updated: total=%d healthy=%d", total, healthy)
	}
	// And the config file must now contain it (survives restart).
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "atr_new") {
		t.Fatalf("key not persisted: %s", b)
	}
	// Rewritten config must still parse cleanly.
	if _, err := config.Load(cfgPath); err != nil {
		t.Fatalf("persisted config is invalid: %v", err)
	}

	// Remove it.
	w = mgmtDo(t, s, "DELETE", "/v0/management/keys/"+resp.ID, nil)
	if w.Code != 200 {
		t.Fatalf("remove: %d %s", w.Code, w.Body.String())
	}
	total, _ = s.pool.Summary()
	if total != 1 {
		t.Fatalf("key not removed from pool: %d", total)
	}
}

func TestAPIKeyAddTakesEffectImmediately(t *testing.T) {
	s, _ := mgmtServer(t)

	// Call the gateway with a key that does not exist yet.
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Atria-Dawn-Preview","messages":[]}`))
	req.Header.Set("Authorization", "Bearer gw-new")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("unknown key must be rejected, got %d", w.Code)
	}

	// Add it through the management API.
	if w := mgmtDo(t, s, "POST", "/v0/management/api-keys", map[string]any{"key": "gw-new"}); w.Code != 200 {
		t.Fatalf("add api key: %d %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Atria-Dawn-Preview","messages":[]}`))
	req.Header.Set("Authorization", "Bearer gw-new")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("new api key must work immediately, got %d %s", w.Code, w.Body.String())
	}
}

func TestAPIKeyRemoveBlocksImmediately(t *testing.T) {
	s, _ := mgmtServer(t)
	call := func(key string) int {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Atria-Dawn-Preview","messages":[]}`))
		req.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		return w.Code
	}
	if call("gw-orig") != 200 {
		t.Fatal("baseline call failed")
	}
	id := config.UpstreamKeyID("gw-orig")
	if w := mgmtDo(t, s, "DELETE", "/v0/management/api-keys/"+id, nil); w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if call("gw-orig") != 401 {
		t.Fatalf("removed key must stop working immediately")
	}
}

func TestStatsAndPorts(t *testing.T) {
	s, _ := mgmtServer(t)
	// Generate some traffic (and an error).
	call := func(key string) int {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"Atria-Dawn-Preview","messages":[]}`))
		req.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		return w.Code
	}
	call("gw-orig")
	call("gw-orig")
	call("nope")

	w := mgmtDo(t, s, "GET", "/v0/management/stats", nil)
	if w.Code != 200 {
		t.Fatalf("stats: %d", w.Code)
	}
	var stats struct {
		Ports     map[string]any `json:"ports"`
		Endpoints map[string]struct {
			Requests int `json:"requests"`
			Errors   int `json:"errors"`
		} `json:"endpoints"`
		KeysTotal   int `json:"keys_total"`
		KeysHealthy int `json:"keys_healthy"`
	}
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats.Ports["http"] != "127.0.0.1:8318" {
		t.Fatalf("ports not reported: %v", stats.Ports)
	}
	ep := stats.Endpoints["/v1/chat/completions"]
	if ep.Requests != 3 || ep.Errors != 1 {
		t.Fatalf("endpoint stats wrong: %+v", ep)
	}
	if stats.KeysTotal != 1 || stats.KeysHealthy != 1 {
		t.Fatalf("key summary wrong: %+v", stats)
	}
}

func TestBadManagementKey(t *testing.T) {
	s, _ := mgmtServer(t)
	req := httptest.NewRequest("POST", "/v0/management/keys", strings.NewReader(`{"key":"x"}`))
	req.Header.Set("X-Management-Key", "wrong")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("wrong mgmt key: got %d", w.Code)
	}
}

func firstChars(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
