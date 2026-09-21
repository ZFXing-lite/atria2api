package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/ZFXing-lite/atria2api/internal/config"
	"github.com/ZFXing-lite/atria2api/internal/proxypool"
)

// management exposes runtime controls plus the web panel. The whole group is
// disabled (404) when remote-management.secret-key is empty.
func (s *Server) management(w http.ResponseWriter, r *http.Request) {
	if !s.mgmtOn.Load() {
		writeJSON(w, http.StatusNotFound, errBody("not found", "not_found"))
		return
	}
	cfg := s.cfg.Load()

	// The panel itself is public HTML; its JS authenticates every API call.
	p := strings.TrimPrefix(r.URL.Path, "/v0/management")
	if p == "" || p == "/" || p == "/panel" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(panelHTML))
		return
	}

	// Credential for the JSON API.
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-Management-Key"))
	}
	if token == "" || !subtle(cfg.Management.SecretKey, token) {
		writeJSON(w, http.StatusUnauthorized, errBody("invalid management key", "invalid_mgmt_key"))
		return
	}
	if !cfg.Management.AllowRemote && !isLoopback(r) {
		writeJSON(w, http.StatusForbidden, errBody("remote management disabled", "remote_forbidden"))
		return
	}

	switch {
	case p == "/endpoints":
		writeJSON(w, http.StatusOK, map[string]any{"endpoints": []string{
			"/config", "/keys", "/keys/bulk", "/api-keys", "/usage", "/stats", "/proxies", "/panel",
		}})
	case p == "/config":
		b, err := cfg.SnapshotJSON()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody(err.Error(), "snapshot_failed"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)

	case p == "/keys" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"keys": s.pool.Status()})
	case p == "/keys" && r.Method == http.MethodPost:
		s.addUpstreamKey(w, r)
	case p == "/keys/bulk" && r.Method == http.MethodPost:
		s.bulkAddUpstreamKeys(w, r)
	case strings.HasPrefix(p, "/keys/") && r.Method == http.MethodDelete:
		id := idFromPath(p)
		if !s.pool.RemoveKey(id) || !s.mutateConfig(func(c *config.Config) bool { return c.RemoveUpstreamKey(id) }) {
			writeJSON(w, http.StatusNotFound, errBody("unknown key id", "unknown_key"))
			return
		}
		slog.Info("upstream key removed", "id", id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case strings.HasPrefix(p, "/keys/") && strings.HasSuffix(p, "/disable"):
		if s.pool.Disable(idFromPath(p)) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		} else {
			writeJSON(w, http.StatusNotFound, errBody("unknown key id", "unknown_key"))
		}
	case strings.HasPrefix(p, "/keys/") && strings.HasSuffix(p, "/enable"):
		if s.pool.Enable(idFromPath(p)) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		} else {
			writeJSON(w, http.StatusNotFound, errBody("unknown key id", "unknown_key"))
		}

	case p == "/api-keys" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"keys": cfg.APIKeysMasked()})
	case p == "/api-keys" && r.Method == http.MethodPost:
		s.addAPIKey(w, r)
	case strings.HasPrefix(p, "/api-keys/") && r.Method == http.MethodDelete:
		id := idFromPath(p)
		if !s.mutateConfig(func(c *config.Config) bool { return c.RemoveAPIKey(id) }) {
			writeJSON(w, http.StatusNotFound, errBody("unknown api key id", "unknown_key"))
			return
		}
		slog.Info("downstream api key removed", "id", id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case p == "/usage":
		if s.metrics == nil {
			writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
			return
		}
		writeJSON(w, http.StatusOK, s.metrics.Snapshot())

	case p == "/stats":
		s.statsHandler(w, r)

	case p == "/proxies":
		writeJSON(w, http.StatusOK, map[string]any{"proxies": proxyStatusList(s.proxies)})

	default:
		writeJSON(w, http.StatusNotFound, errBody("unknown management endpoint", "not_found"))
	}
}

func proxyStatusList(p *proxypool.Pool) []proxypool.Status {
	if p == nil {
		return nil
	}
	return p.Status()
}

// addUpstreamKey adds an Atria key at runtime: pool first (immediate effect),
// then config (survives restart), then persist.
func (s *Server) addUpstreamKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key    string `json:"key"`
		Weight int    `json:"weight"`
		Proxy  string `json:"proxy"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody(err.Error(), "bad_request"))
		return
	}
	req.Key = strings.TrimSpace(req.Key)
	if req.Key == "" {
		writeJSON(w, http.StatusBadRequest, errBody("key is required", "bad_request"))
		return
	}
	id := s.pool.AddKey(req.Key, req.Weight, req.Proxy)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid key", "bad_request"))
		return
	}
	if !s.mutateConfig(func(c *config.Config) bool {
		c.AddUpstreamKey(req.Key, req.Weight, req.Proxy)
		return true
	}) {
		writeJSON(w, http.StatusInternalServerError, errBody("config not persisted", "persist_failed"))
		return
	}
	slog.Info("upstream key added", "id", id, "weight", req.Weight)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// bulkAddUpstreamKeys imports many keys at once. Each line is one key; blank
// lines and #-comments are ignored and duplicates are collapsed.
func (s *Server) bulkAddUpstreamKeys(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keys   []string `json:"keys"`
		Weight int      `json:"weight"`
		Proxy  string   `json:"proxy"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody(err.Error(), "bad_request"))
		return
	}
	if weight := req.Weight; weight <= 0 {
		req.Weight = 1
	}
	proxy := strings.TrimSpace(req.Proxy)

	seen := map[string]bool{}
	added, updated, skipped := 0, 0, 0
	var ids []string
	for _, raw := range req.Keys {
		k := strings.TrimSpace(raw)
		if k == "" || strings.HasPrefix(k, "#") {
			skipped++
			continue
		}
		if seen[k] {
			skipped++
			continue
		}
		seen[k] = true
		existed := s.pool.Has(k)
		id := s.pool.AddKey(k, req.Weight, proxy)
		if id == "" {
			skipped++
			continue
		}
		if existed {
			updated++
		} else {
			added++
			ids = append(ids, id)
		}
		s.mutateConfig(func(c *config.Config) bool {
			c.AddUpstreamKey(k, req.Weight, proxy)
			return true
		})
	}
	slog.Info("bulk import", "added", added, "updated", updated, "skipped", skipped)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "added": added, "updated": updated, "skipped": skipped, "ids": ids,
	})
}

// addAPIKey adds a downstream key clients can use to call this gateway.
func (s *Server) addAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody(err.Error(), "bad_request"))
		return
	}
	if !s.mutateConfig(func(c *config.Config) bool { return c.AddAPIKey(req.Key) }) {
		writeJSON(w, http.StatusConflict, errBody("empty or duplicate api key", "duplicate"))
		return
	}
	slog.Info("downstream api key added", "id", config.UpstreamKeyID(req.Key))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// mutateConfig applies fn to a copy of the live config, publishes it to all
// components and persists it atomically.
func (s *Server) mutateConfig(fn func(*config.Config) bool) bool {
	cur := s.cfg.Load()
	cp := *cur
	cp.Upstream.Keys = append([]config.UpstreamKey(nil), cur.Upstream.Keys...)
	cp.APIKeys = append([]string(nil), cur.APIKeys...)
	cp.Proxy.SOCKS5 = append([]config.ProxyEntry(nil), cur.Proxy.SOCKS5...)
	if !fn(&cp) {
		return false
	}
	s.Update(&cp)
	if err := cp.Save(s.configPath); err != nil {
		slog.Error("persist config failed", "err", err)
		// The live config is already updated; a failed write only means the
		// change may not survive a restart.
		return true
	}
	return true
}

// statsHandler returns ports, endpoint call counters and health for the panel.
func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Load()
	total, healthy := s.pool.Summary()
	resp := map[string]any{
		"ports": map[string]any{
			"http":          cfg.ListenAddr(),
			"tls":           cfg.TLS.Enable,
			"pprof":         "127.0.0.1:8319",
			"auth_required": cfg.AuthRequired(),
			"mgmt_remote":   cfg.Management.AllowRemote,
			"upstream":      cfg.Upstream.BaseURL,
			"default_model": cfg.Upstream.DefaultModel,
		},
		"endpoints":     s.stats.Snapshot(),
		"keys_total":    total,
		"keys_healthy":  healthy,
		"proxies_total": len(cfg.Proxy.SOCKS5),
	}
	if s.proxies != nil {
		resp["proxies"] = s.proxies.Status()
		resp["proxies_total"] = len(s.proxies.Status())
	}
	writeJSON(w, http.StatusOK, resp)
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(v)
}

// idFromPath extracts the key id from "/keys/<id>/disable" style paths.
func idFromPath(p string) string {
	for _, pref := range []string{"/keys/", "/api-keys/"} {
		if strings.HasPrefix(p, pref) {
			rest := p[len(pref):]
			for _, suf := range []string{"/disable", "/enable"} {
				rest = strings.TrimSuffix(rest, suf)
			}
			return path.Clean("/" + rest)[1:]
		}
	}
	return path.Clean("/" + p)[1:]
}

func isLoopback(r *http.Request) bool {
	host := r.Host
	if h := strings.Split(host, ":")[0]; h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	ip = strings.Trim(ip, "[]")
	return ip == "127.0.0.1" || ip == "::1" || ip == ""
}

func subtle(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var d byte
	for i := 0; i < len(a); i++ {
		d |= a[i] ^ b[i]
	}
	return d == 0
}
