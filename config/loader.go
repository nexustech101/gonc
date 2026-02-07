package config

// loader.go - configuration loading from environment variables.
//
// Precedence order (highest wins):
//   1. CLI flags  (handled by cmd/root.go)
//   2. Environment variables  (this file)
//   3. Defaults   (defaults.go)

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// ── Environment variable mapping ─────────────────────────────────────
//
// Every supported env var uses the GONC_ prefix.  Boolean values
// accept "1", "true", "yes" (case-insensitive).

// LoadFromEnv overlays environment variables onto cfg.  Only non-empty
// env vars override the existing value.  This should be called BEFORE
// CLI flag parsing so that flags take precedence.
func LoadFromEnv(cfg *Config) {
	if v := os.Getenv("GONC_HOST"); v != "" {
		cfg.Host = v
	}
	if v := envInt("GONC_PORT"); v > 0 {
		cfg.LocalPort = v
	}
	if envBool("GONC_LISTEN") {
		cfg.Listen = true
	}
	if envBool("GONC_UDP") {
		cfg.UDP = true
	}
	if envBool("GONC_NO_DNS") {
		cfg.NoDNS = true
	}
	if envBool("GONC_KEEP_OPEN") {
		cfg.KeepOpen = true
	}
	if v := envInt("GONC_TIMEOUT"); v > 0 {
		cfg.Timeout = secondsDuration(v)
	}

	// SSH tunnel
	if v := os.Getenv("GONC_TUNNEL"); v != "" {
		cfg.TunnelSpec = v
	}
	if v := os.Getenv("GONC_SSH_KEY"); v != "" {
		cfg.SSHKeyPath = v
	}
	if envBool("GONC_SSH_PASSWORD") {
		cfg.SSHPassword = true
	}
	if envBool("GONC_SSH_AGENT") {
		cfg.UseSSHAgent = true
	}
	if envBool("GONC_STRICT_HOSTKEY") {
		cfg.StrictHostKey = true
	}
	if v := os.Getenv("GONC_KNOWN_HOSTS"); v != "" {
		cfg.KnownHostsPath = v
	}

	// Reverse tunnel
	if v := os.Getenv("GONC_REVERSE_TUNNEL"); v != "" {
		cfg.ReverseTunnelSpec = v
	}
	if v := envInt("GONC_REMOTE_PORT"); v > 0 {
		cfg.RemotePort = v
	}
	if v := os.Getenv("GONC_REMOTE_BIND_ADDRESS"); v != "" {
		cfg.RemoteBindAddress = v
	}
	if v := envInt("GONC_KEEP_ALIVE"); v > 0 {
		cfg.KeepAliveInterval = v
	}
	if envBool("GONC_AUTO_RECONNECT") {
		cfg.AutoReconnect = true
	}

	// Output
	if v := envInt("GONC_VERBOSE"); v > 0 {
		cfg.Verbose = v
	}
}

// ── helpers ──────────────────────────────────────────────────────────

func envInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func envBool(key string) bool {
	v := strings.ToLower(os.Getenv(key))
	return v == "1" || v == "true" || v == "yes"
}

func secondsDuration(sec int) time.Duration {
	return time.Duration(sec) * time.Second
}
