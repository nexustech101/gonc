// Package config defines the runtime configuration for gonc and provides
// helpers for parsing tunnel specifications and port ranges.
package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config holds every tuneable for a single gonc session.
type Config struct {
	// ── Connection ───────────────────────────────────────────────────
	Host      string
	Port      int         // primary destination port
	Ports     []PortRange // all destination port specs (scanning)
	LocalPort int         // -p: local bind port
	Listen    bool
	UDP       bool
	Timeout   time.Duration
	KeepOpen  bool
	NoDNS     bool

	// ── SSH tunnel ───────────────────────────────────────────────────
	TunnelSpec      string // raw user@host[:port] from -T
	TunnelEnabled   bool
	TunnelUser      string
	TunnelHost      string
	TunnelPort      int
	SSHKeyPath      string
	SSHPassword     bool // true → prompt interactively
	UseSSHAgent     bool
	StrictHostKey   bool
	KnownHostsPath  string
	TunnelLocalPort int

	// ── Execution ────────────────────────────────────────────────────
	Execute string // -e: program path
	Command string // -c: shell command

	// ── Output ───────────────────────────────────────────────────────
	Verbose int
	ZeroIO  bool
}

// ── Port helpers ─────────────────────────────────────────────────────

// PortRange is an inclusive start–end pair.
type PortRange struct {
	Start int
	End   int
}

// Expand returns every port in the range.
func (pr PortRange) Expand() []int {
	out := make([]int, 0, pr.End-pr.Start+1)
	for p := pr.Start; p <= pr.End; p++ {
		out = append(out, p)
	}
	return out
}

// AllPorts flattens every PortRange into a single slice.
func (c *Config) AllPorts() []int {
	var out []int
	for _, pr := range c.Ports {
		out = append(out, pr.Expand()...)
	}
	return out
}

// ParsePortSpec accepts "80", "80-90", or "http" (numeric only for now).
func ParsePortSpec(spec string) (PortRange, error) {
	if strings.Contains(spec, "-") {
		parts := strings.SplitN(spec, "-", 2)
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return PortRange{}, fmt.Errorf("invalid port range start %q", parts[0])
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return PortRange{}, fmt.Errorf("invalid port range end %q", parts[1])
		}
		if start < 1 || end > 65535 || start > end {
			return PortRange{}, fmt.Errorf("invalid port range %d-%d", start, end)
		}
		return PortRange{Start: start, End: end}, nil
	}

	port, err := strconv.Atoi(spec)
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid port %q", spec)
	}
	if port < 1 || port > 65535 {
		return PortRange{}, fmt.Errorf("port %d out of range 1-65535", port)
	}
	return PortRange{Start: port, End: port}, nil
}

// ── Tunnel-spec parser ───────────────────────────────────────────────

// tunnelRe matches [user@]host[:port].
var tunnelRe = regexp.MustCompile(`^(?:([^@]+)@)?([^:]+)(?::(\d+))?$`)

// ParseTunnelSpec extracts user, host, and port from a string such as
// "admin@bastion.example.com:2222".  Port defaults to 22.
func ParseTunnelSpec(spec string) (user, host string, port int, err error) {
	m := tunnelRe.FindStringSubmatch(spec)
	if m == nil {
		return "", "", 0, fmt.Errorf("invalid tunnel spec %q – expected [user@]host[:port]", spec)
	}
	user = m[1]
	host = m[2]
	port = 22
	if m[3] != "" {
		port, err = strconv.Atoi(m[3])
		if err != nil || port < 1 || port > 65535 {
			return "", "", 0, fmt.Errorf("invalid tunnel port %q", m[3])
		}
	}
	if host == "" {
		return "", "", 0, fmt.Errorf("tunnel host is required")
	}
	return user, host, port, nil
}

// ── Validation ───────────────────────────────────────────────────────

// Validate checks that the configuration is internally consistent.
func (c *Config) Validate() error {
	if c.Listen {
		if c.LocalPort == 0 {
			return fmt.Errorf("listen mode requires -p <port>")
		}
		if c.ZeroIO {
			return fmt.Errorf("listen mode and zero-I/O mode are mutually exclusive")
		}
		if c.TunnelEnabled {
			return fmt.Errorf("listen mode through an SSH tunnel is not yet supported")
		}
	} else {
		if c.Host == "" {
			return fmt.Errorf("hostname is required (use --help for usage)")
		}
		if c.Port == 0 && len(c.Ports) == 0 {
			return fmt.Errorf("destination port is required")
		}
	}

	if c.Execute != "" && c.Command != "" {
		return fmt.Errorf("-e and -c are mutually exclusive")
	}

	if c.UDP && c.TunnelEnabled {
		return fmt.Errorf("UDP is not supported through SSH tunnels")
	}

	if c.TunnelEnabled && c.TunnelHost == "" {
		return fmt.Errorf("tunnel host is required")
	}

	return nil
}
