package errors

import (
	"fmt"
	"io"
	"net"
	"testing"
)

func TestNetworkError_Format(t *testing.T) {
	tests := []struct {
		name string
		err  NetworkError
		want string
	}{
		{
			name: "retryable",
			err:  NetworkError{Op: "dial", Addr: "example.com:80", Err: io.EOF, Retryable: true},
			want: "dial example.com:80: EOF (retryable)",
		},
		{
			name: "non-retryable",
			err:  NetworkError{Op: "listen", Addr: ":8080", Err: fmt.Errorf("bind failed")},
			want: "listen :8080: bind failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNetworkError_Unwrap(t *testing.T) {
	err := &NetworkError{Op: "dial", Addr: "x", Err: io.EOF}
	if !Is(err, io.EOF) {
		t.Error("should unwrap to io.EOF")
	}
}

func TestSSHError_Format(t *testing.T) {
	err := WrapSSH("handshake", "bastion.example.com", 22, fmt.Errorf("connection refused"))
	want := "ssh handshake bastion.example.com:22: connection refused"
	if got := err.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSSHError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("auth fail")
	err := WrapSSH("auth", "host", 22, inner)
	if !Is(err, inner) {
		t.Error("should unwrap to inner error")
	}
}

func TestConfigError_Format(t *testing.T) {
	tests := []struct {
		name string
		err  ConfigError
		want string
	}{
		{
			name: "with value and hint",
			err: ConfigError{
				Field:   "port",
				Value:   99999,
				Message: "out of range 1-65535",
				Hint:    "use a port between 1 and 65535",
			},
			want: "config: --port=99999: out of range 1-65535\n  hint: use a port between 1 and 65535",
		},
		{
			name: "missing value no hint",
			err: ConfigError{
				Field:   "remote-port",
				Message: "required with -R",
			},
			want: "config: --remote-port: required with -R",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	err := Wrap("dial", "10.0.0.1:22", inner)

	if err.Op != "dial" || err.Addr != "10.0.0.1:22" {
		t.Errorf("wrong fields: Op=%q Addr=%q", err.Op, err.Addr)
	}
	if !Is(err, inner) {
		t.Error("should unwrap to inner error")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"retryable network", &NetworkError{Op: "dial", Addr: "x", Err: io.EOF, Retryable: true}, true},
		{"non-retryable network", &NetworkError{Op: "dial", Addr: "x", Err: io.EOF, Retryable: false}, false},
		{"plain error", fmt.Errorf("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTemporary(t *testing.T) {
	ne := &NetworkError{Op: "read", Addr: "x", Err: io.EOF, Retryable: true}
	if !IsTemporary(ne) {
		t.Error("expected temporary")
	}
}

func TestClassifyRetryable_NetOpError(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &net.DNSError{IsTemporary: true},
	}
	if !classifyRetryable(opErr) {
		t.Error("temporary OpError should be retryable")
	}
}

func TestSentinels(t *testing.T) {
	// Verify sentinel errors are distinct.
	sentinels := []error{
		ErrTunnelClosed, ErrNotConnected, ErrCircuitOpen,
		ErrTimeout, ErrAuthFailed, ErrHostKeyMismatch,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && Is(a, b) {
				t.Errorf("sentinel %d and %d should not match", i, j)
			}
		}
	}
}
