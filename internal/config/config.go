// Package config loads, validates and hot-reloads the atria2api gateway configuration.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the full gateway configuration. All durations are strings accepted
// by time.ParseDuration (e.g. "60s", "10m", "1h").
type Config struct {
	Host       string     `yaml:"host"`
	Port       int        `yaml:"port"`
	TLS        TLS        `yaml:"tls"`
	APIKeys    []string   `yaml:"api-keys"`
	Upstream   Upstream   `yaml:"upstream"`
	Proxy      Proxy      `yaml:"proxy"`
	RateLimit  RateLimit  `yaml:"rate-limit"`
	Log        Log        `yaml:"log"`
	Metrics    Metrics    `yaml:"metrics"`
	Management Management `yaml:"remote-management"`
}

type TLS struct {
	Enable bool   `yaml:"enable"`
	Cert   string `yaml:"cert"`
	Key    string `yaml:"key"`
}

// UpstreamKey is one Atria API key (atr_xxx). Weight is used by the weighted
// round-robin picker; <=0 means 1.
type UpstreamKey struct {
	Key    string `yaml:"key"`
	Weight int    `yaml:"weight"`
	Proxy  string `yaml:"proxy"` // optional per-key proxy override (URL or "none")
}

// Upstream describes the single Atria API endpoint. The base URL is shared by
// all three interfaces per the API documentation.
type Upstream struct {
	BaseURL        string        `yaml:"base-url"`
	Keys           []UpstreamKey `yaml:"keys"`
	Timeout        Duration      `yaml:"timeout"`
	ConnectTimeout Duration      `yaml:"connect-timeout"`
	DefaultModel   string        `yaml:"default-model"`
	// ForceModel rewrites request body "model" to DefaultModel so clients that
	// send a stale/other model id still work (Atria only exposes one model).
	ForceModel bool `yaml:"force-model"`
}

type Proxy struct {
	// Policy: round-robin | random | sticky-key
	Policy       string       `yaml:"policy"`
	HealthEvery  Duration     `yaml:"health-every"`
	FailCooldown Duration     `yaml:"fail-cooldown"`
	SOCKS5       []ProxyEntry `yaml:"socks5"`
}

type ProxyEntry struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

type RateLimit struct {
	// RespectHeader parses upstream x-rpm-limit / x-rpm-remaining / Retry-After.
	RespectHeader bool `yaml:"respect-header"`
	// MinRPMReserve skips a key when its observed x-rpm-remaining is below this.
	MinRPMReserve int `yaml:"min-rpm-reserve"`
	// Cooldown429 is used when the upstream gives no Retry-After.
	Cooldown429 Duration `yaml:"cooldown-429"`
	// Cooldown5xx applies to 5xx responses.
	Cooldown5xx Duration `yaml:"cooldown-5xx"`
	// ErrThreshold consecutive errors before a key is cooled down.
	ErrThreshold int `yaml:"err-threshold"`
	// ErrCooldown duration of the consecutive-error cooldown.
	ErrCooldown Duration `yaml:"err-cooldown"`
	// DisableOn401 permanently disables a key whose key is invalid/revoked.
	DisableOn401 bool `yaml:"disable-on-401"`
	// MaxRetries is the number of cross-key retries per request.
	MaxRetries int `yaml:"max-retries"`
	// RetryOn lists upstream HTTP status codes that trigger a retry.
	RetryOn []int `yaml:"retry-on"`
	// Backoff is the base sleep between retries (exponential: backoff*2^attempt).
	Backoff Duration `yaml:"backoff"`
}

type Log struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // text | json
}

type Metrics struct {
	Enabled    bool     `yaml:"enabled"`
	StateFile  string   `yaml:"state-file"`
	FlushEvery Duration `yaml:"flush-every"`
}

type Management struct {
	AllowRemote bool   `yaml:"allow-remote"`
	SecretKey   string `yaml:"secret-key"`
}

// Duration is a time.Duration that (un)marshals from/to a duration string.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	return d.Set(s)
}

func (d Duration) String() string { return time.Duration(d).String() }

// Set parses a duration string; used by env overrides too.
func (d *Duration) Set(s string) error {
	if s == "" {
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

const (
	DefaultBaseURL    = "https://api.atria-asi.ai"
	DefaultModel      = "Atria-Dawn-Preview"
	DefaultListenPort = 8318
)

// Default returns a usable zero-config configuration.
func Default() *Config {
	return &Config{
		Host: "",
		Port: DefaultListenPort,
		Upstream: Upstream{
			BaseURL:        DefaultBaseURL,
			DefaultModel:   DefaultModel,
			ForceModel:     true,
			Timeout:        Duration(10 * time.Minute),
			ConnectTimeout: Duration(15 * time.Second),
		},
		Proxy: Proxy{
			Policy:       "round-robin",
			HealthEvery:  Duration(30 * time.Second),
			FailCooldown: Duration(30 * time.Second),
		},
		RateLimit: RateLimit{
			RespectHeader: true,
			MinRPMReserve: 1,
			Cooldown429:   Duration(60 * time.Second),
			Cooldown5xx:   Duration(30 * time.Second),
			ErrThreshold:  3,
			ErrCooldown:   Duration(10 * time.Minute),
			DisableOn401:  true,
			MaxRetries:    3,
			RetryOn:       []int{429, 500, 502, 503, 504},
			Backoff:       Duration(time.Second),
		},
		Log: Log{Level: "info", Format: "text"},
		Metrics: Metrics{
			Enabled:    true,
			FlushEvery: Duration(5 * time.Second),
		},
	}
}

// Load reads path (YAML). A missing path is fine: defaults are returned and the
// file is not created. ATRIA2API_* environment variables override fields.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config: %w", err)
			}
			slog.Warn("config file not found, using defaults", "path", path)
		} else if len(raw) > 0 {
			dec := yaml.NewDecoder(strings.NewReader(string(raw)))
			dec.KnownFields(true)
			if err := dec.Decode(cfg); err != nil {
				return nil, fmt.Errorf("parse config %s: %w", path, err)
			}
		}
	}
	if err := applyEnv(cfg); err != nil {
		return nil, err
	}
	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) normalize() error {
	if c.Port <= 0 {
		c.Port = DefaultListenPort
	}
	c.Upstream.BaseURL = strings.TrimRight(strings.TrimSpace(c.Upstream.BaseURL), "/")
	if c.Upstream.BaseURL == "" {
		c.Upstream.BaseURL = DefaultBaseURL
	}
	if c.Upstream.DefaultModel == "" {
		c.Upstream.DefaultModel = DefaultModel
	}
	if c.Upstream.Timeout <= 0 {
		c.Upstream.Timeout = Duration(10 * time.Minute)
	}
	if c.Upstream.ConnectTimeout <= 0 {
		c.Upstream.ConnectTimeout = Duration(15 * time.Second)
	}
	if len(c.Upstream.Keys) == 0 {
		return fmt.Errorf("upstream.keys is empty: add at least one atr_ key (see https://api.atria-asi.ai/docs)")
	}
	seen := map[string]bool{}
	for i := range c.Upstream.Keys {
		k := strings.TrimSpace(c.Upstream.Keys[i].Key)
		if k == "" {
			return fmt.Errorf("upstream.keys[%d].key is empty", i)
		}
		if seen[k] {
			return fmt.Errorf("upstream.keys[%d].key is duplicated: %s", i, mask(k))
		}
		seen[k] = true
		if c.Upstream.Keys[i].Weight <= 0 {
			c.Upstream.Keys[i].Weight = 1
		}
	}
	switch strings.ToLower(c.Proxy.Policy) {
	case "", "round-robin", "random", "sticky-key":
	default:
		return fmt.Errorf("proxy.policy %q is invalid (use round-robin|random|sticky-key)", c.Proxy.Policy)
	}
	if c.RateLimit.MaxRetries < 0 {
		c.RateLimit.MaxRetries = 0
	}
	if c.RateLimit.Cooldown429 <= 0 {
		c.RateLimit.Cooldown429 = Duration(time.Minute)
	}
	if c.RateLimit.Cooldown5xx <= 0 {
		c.RateLimit.Cooldown5xx = Duration(30 * time.Second)
	}
	if c.RateLimit.ErrThreshold <= 0 {
		c.RateLimit.ErrThreshold = 3
	}
	if c.RateLimit.ErrCooldown <= 0 {
		c.RateLimit.ErrCooldown = Duration(10 * time.Minute)
	}
	if c.RateLimit.Backoff < 0 {
		c.RateLimit.Backoff = 0
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
	if c.Metrics.Enabled && c.Metrics.StateFile == "" {
		c.Metrics.StateFile = "state.json"
	}
	if c.Metrics.FlushEvery <= 0 {
		c.Metrics.FlushEvery = Duration(5 * time.Second)
	}
	return nil
}

// ListenAddr returns host:port.
func (c *Config) ListenAddr() string { return fmt.Sprintf("%s:%d", c.Host, c.Port) }

// UpstreamKeysFlat returns the key strings in config order.
func (c *Config) UpstreamKeysFlat() []string {
	out := make([]string, 0, len(c.Upstream.Keys))
	for _, k := range c.Upstream.Keys {
		out = append(out, k.Key)
	}
	return out
}

// AuthRequired reports whether downstream api-keys gate access.
func (c *Config) AuthRequired() bool {
	for _, k := range c.APIKeys {
		if strings.TrimSpace(k) != "" {
			return true
		}
	}
	return false
}

// IsAuthorized checks a downstream credential against configured api-keys.
// Constant-time-ish compare; empty config = open gateway.
func (c *Config) IsAuthorized(token string) bool {
	if !c.AuthRequired() {
		return true
	}
	for _, k := range c.APIKeys {
		if subtleEqual(strings.TrimSpace(k), token) {
			return true
		}
	}
	return false
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// --- env overrides -------------------------------------------------------

func applyEnv(c *Config) error {
	get := func(name string) string { return os.Getenv(name) }
	if v := strings.TrimSpace(get("ATRIA2API_LISTEN")); v != "" {
		c.Host = v
	}
	if v := strings.TrimSpace(get("ATRIA2API_PORT")); v != "" {
		var p int
		if _, err := fmt.Sscanf(v, "%d", &p); err != nil {
			return fmt.Errorf("ATRIA2API_PORT: %w", err)
		}
		c.Port = p
	}
	if v := strings.TrimSpace(get("ATRIA2API_BASE_URL")); v != "" {
		c.Upstream.BaseURL = v
	}
	if v := strings.TrimSpace(get("ATRIA2API_MODEL")); v != "" {
		c.Upstream.DefaultModel = v
	}
	// Comma/space separated keys; also accepts the raw key list.
	if v := strings.TrimSpace(get("ATRIA2API_KEYS")); v != "" {
		for _, k := range splitList(v) {
			c.Upstream.Keys = append(c.Upstream.Keys, UpstreamKey{Key: k})
		}
	}
	if v := strings.TrimSpace(get("ATRIA2API_API_KEYS")); v != "" {
		c.APIKeys = splitList(v)
	}
	if v := strings.TrimSpace(get("ATRIA2API_PROXIES")); v != "" {
		for _, u := range splitList(v) {
			c.Proxy.SOCKS5 = append(c.Proxy.SOCKS5, ProxyEntry{URL: u})
		}
	}
	if v := strings.TrimSpace(get("ATRIA2API_LOG_LEVEL")); v != "" {
		c.Log.Level = v
	}
	if v := strings.TrimSpace(get("ATRIA2API_MGMT_KEY")); v != "" {
		c.Management.SecretKey = v
	}
	return nil
}

func splitList(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// --- hot reload ----------------------------------------------------------

// Watcher reloads the config file on SIGHUP and on file change, calling cb with
// the new configuration. Errors are logged, never fatal.
type Watcher struct {
	path string
	mu   sync.Mutex
	cur  *Config
	cb   func(*Config)
	stop chan struct{}
}

func NewWatcher(path string, cb func(*Config)) *Watcher {
	return &Watcher{path: path, cb: cb, stop: make(chan struct{})}
}

func (w *Watcher) Current() *Config {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cur
}

// Run blocks until ctx is done.
func (w *Watcher) Run(ctx context.Context, initial *Config) {
	w.mu.Lock()
	w.cur = initial
	w.mu.Unlock()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	mod := fileMod(w.path)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			if m := fileMod(w.path); !m.IsZero() && mod != m {
				mod = m
				w.reload()
			}
		}
	}
}

func (w *Watcher) reload() {
	cfg, err := Load(w.path)
	if err != nil {
		slog.Error("config reload failed", "err", err)
		return
	}
	w.mu.Lock()
	w.cur = cfg
	w.mu.Unlock()
	slog.Info("config reloaded", "path", w.path)
	if w.cb != nil {
		w.cb(cfg)
	}
}

func fileMod(path string) time.Time {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

// Mask hides all but the first 4 and last 2 chars of a secret, for logs.
func mask(s string) string {
	if len(s) <= 6 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-6) + s[len(s)-2:]
}

// MaskKey is the exported masking helper used across packages.
func MaskKey(s string) string { return mask(s) }

// SnapshotJSON serializes the non-secret parts of the config for the management API.
func (c *Config) SnapshotJSON() ([]byte, error) {
	type outKey struct {
		Key    string `json:"key"`
		Weight int    `json:"weight"`
		Proxy  string `json:"proxy,omitempty"`
	}
	cp := *c
	cp.APIKeys = nil
	keys := make([]outKey, 0, len(c.Upstream.Keys))
	for _, k := range c.Upstream.Keys {
		keys = append(keys, outKey{Key: mask(k.Key), Weight: k.Weight, Proxy: k.Proxy})
	}
	cp.Upstream.Keys = nil
	b, err := json.MarshalIndent(&struct {
		Host         string     `json:"host"`
		Port         int        `json:"port"`
		TLS          TLS        `json:"tls"`
		Upstream     Upstream   `json:"upstream"`
		Proxy        Proxy      `json:"proxy"`
		RateLimit    RateLimit  `json:"rate-limit"`
		Log          Log        `json:"log"`
		Metrics      Metrics    `json:"metrics"`
		Management   Management `json:"remote-management"`
		KeysMasked   []outKey   `json:"upstream-keys-masked"`
		AuthRequired bool       `json:"auth-required"`
	}{
		Host: cp.Host, Port: cp.Port, TLS: cp.TLS, Upstream: cp.Upstream,
		Proxy: cp.Proxy, RateLimit: cp.RateLimit, Log: cp.Log, Metrics: cp.Metrics,
		Management: cp.Management, KeysMasked: keys, AuthRequired: c.AuthRequired(),
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ResolvePath expands ~ and makes a relative path absolute against baseDir.
func ResolvePath(p, baseDir string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}
