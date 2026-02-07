package config

import "time"

// ── Default values ───────────────────────────────────────────────────
//
// All tuneable defaults live here so they are easy to audit and reuse
// across CLI flags, config file parsing, and environment variable
// loading.

const (
	// DefaultSSHPort is the standard SSH port.
	DefaultSSHPort = 22

	// DefaultLocalAddress is the address used for local service binding.
	DefaultLocalAddress = "127.0.0.1"

	// DefaultKeepAliveInterval is the SSH keepalive interval in seconds.
	DefaultKeepAliveInterval = 30

	// DefaultScanTimeout is the per-port timeout for port scanning.
	DefaultScanTimeout = 3 * time.Second

	// DefaultMaxConcurrentScans limits the number of simultaneous scan
	// goroutines to prevent resource exhaustion.
	DefaultMaxConcurrentScans = 100

	// DefaultConnTimeout is the TCP/SSH connection timeout.
	DefaultConnTimeout = 30 * time.Second

	// DefaultMaxReconnectAttempts is how many times to retry after a
	// tunnel disconnect.
	DefaultMaxReconnectAttempts = 10

	// DefaultMaxReconnectBackoff caps the exponential backoff between
	// reconnection attempts.
	DefaultMaxReconnectBackoff = 60 * time.Second

	// DefaultGracePeriod is how long Close waits for handlers to finish.
	DefaultGracePeriod = 5 * time.Second
)
