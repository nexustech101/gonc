// Package errors provides domain-specific error types for gonc.
//
// These types carry structured context (operation, address, retryability)
// that helps callers decide how to handle failures and provides better
// diagnostics than plain string wrapping.
package errors

import (
	"errors"
	"fmt"
	"net"
)

// ── Sentinel errors ──────────────────────────────────────────────────

var (
	ErrTunnelClosed    = errors.New("tunnel is closed")
	ErrNotConnected    = errors.New("not connected")
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTimeout         = errors.New("operation timed out")
	ErrAuthFailed      = errors.New("authentication failed")
	ErrHostKeyMismatch = errors.New("host key mismatch")
)

// ── Structured error types ───────────────────────────────────────────

// NetworkError represents a failure in a network operation.
type NetworkError struct {
	Op        string // operation: "dial", "listen", "accept", "write", "read"
	Addr      string // network address involved
	Err       error  // underlying error
	Retryable bool   // whether the caller should retry
}

func (e *NetworkError) Error() string {
	s := fmt.Sprintf("%s %s: %v", e.Op, e.Addr, e.Err)
	if e.Retryable {
		s += " (retryable)"
	}
	return s
}

func (e *NetworkError) Unwrap() error { return e.Err }

// SSHError represents an SSH-specific failure with host context.
type SSHError struct {
	Op   string // "handshake", "auth", "channel", "forward"
	Host string
	Port int
	Err  error
}

func (e *SSHError) Error() string {
	return fmt.Sprintf("ssh %s %s:%d: %v", e.Op, e.Host, e.Port, e.Err)
}

func (e *SSHError) Unwrap() error { return e.Err }

// ConfigError represents an invalid configuration value.
type ConfigError struct {
	Field   string      // config field name
	Value   interface{} // the invalid value (nil if missing)
	Message string      // human-readable explanation
	Hint    string      // suggestion for the user (optional)
}

func (e *ConfigError) Error() string {
	msg := fmt.Sprintf("config: --%s", e.Field)
	if e.Value != nil {
		msg += fmt.Sprintf("=%v", e.Value)
	}
	msg += ": " + e.Message
	if e.Hint != "" {
		msg += "\n  hint: " + e.Hint
	}
	return msg
}

// ── Constructors ─────────────────────────────────────────────────────

// Wrap creates a NetworkError, automatically detecting retryability
// from the underlying error.
func Wrap(op, addr string, err error) *NetworkError {
	return &NetworkError{
		Op:        op,
		Addr:      addr,
		Err:       err,
		Retryable: classifyRetryable(err),
	}
}

// WrapSSH creates an SSHError.
func WrapSSH(op, host string, port int, err error) *SSHError {
	return &SSHError{Op: op, Host: host, Port: port, Err: err}
}

// ── Classification helpers ───────────────────────────────────────────

// IsRetryable reports whether err is worth retrying.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var ne *NetworkError
	if errors.As(err, &ne) {
		return ne.Retryable
	}
	return classifyRetryable(err)
}

// IsTemporary reports whether err represents a temporary condition.
func IsTemporary(err error) bool {
	var ne *NetworkError
	if errors.As(err, &ne) {
		return ne.Retryable // temporary ≈ retryable for network errors
	}
	return classifyRetryable(err)
}

// classifyRetryable inspects standard library error types.
func classifyRetryable(err error) bool {
	if err == nil {
		return false
	}
	// net.OpError with Temporary() hint
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Temporary() //nolint:staticcheck // Temporary is deprecated but still useful
	}
	// DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary() //nolint:staticcheck
	}
	return false
}

// ── Re-exports for convenience ───────────────────────────────────────
//
// These allow callers to use gonc/internal/errors as a drop-in
// replacement for the standard library in common operations.

// As is [errors.As].
func As(err error, target interface{}) bool { return errors.As(err, target) }

// Is is [errors.Is].
func Is(err, target error) bool { return errors.Is(err, target) }

// New is [errors.New].
func New(text string) error { return errors.New(text) }

// Unwrap is [errors.Unwrap].
func Unwrap(err error) error { return errors.Unwrap(err) }

// Join is [errors.Join].
func Join(errs ...error) error { return errors.Join(errs...) }
