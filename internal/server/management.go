package server

import (
	"net/http"
	"path"
	"strings"
)

// management exposes a small, safe subset of runtime controls. It is entirely
// disabled (404) when remote-management.secret-key is empty.
func (s *Server) management(w http.ResponseWriter, r *http.Request) {
	if !s.mgmtOn.Load() {
		writeJSON(w, http.StatusNotFound, errBody("not found", "not_found"))
		return
	}
	cfg := s.cfg.Load()

	// Credential.
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-Management-Key"))
	}
	if token == "" || !subtle(cfg.Management.SecretKey, token) {
		writeJSON(w, http.StatusUnauthorized, errBody("invalid management key", "invalid_mgmt_key"))
		return
	}

	// Loopback-only unless explicitly opened up.
	if !cfg.Management.AllowRemote && !isLoopback(r) {
		writeJSON(w, http.StatusForbidden, errBody("remote management disabled", "remote_forbidden"))
		return
	}

	p := strings.TrimPrefix(r.URL.Path, "/v0/management")
	switch {
	case p == "" || p == "/":
		writeJSON(w, http.StatusOK, map[string]any{
			"endpoints": []string{"/config", "/keys", "/usage"},
		})
	case p == "/config":
		b, err := cfg.SnapshotJSON()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody(err.Error(), "snapshot_failed"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	case p == "/keys":
		writeJSON(w, http.StatusOK, map[string]any{"keys": s.pool.Status()})
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
	case p == "/usage":
		if s.metrics == nil {
			writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
			return
		}
		writeJSON(w, http.StatusOK, s.metrics.Snapshot())
	default:
		writeJSON(w, http.StatusNotFound, errBody("unknown management endpoint", "not_found"))
	}
}

// idFromPath extracts the key id from "/keys/<id>/disable".
func idFromPath(p string) string {
	rest := strings.TrimPrefix(p, "/keys/")
	rest = strings.TrimSuffix(rest, "/disable")
	rest = strings.TrimSuffix(rest, "/enable")
	return path.Clean("/" + rest)[1:]
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
