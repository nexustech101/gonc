// Package config defines the runtime configuration for gonc and provides
// helpers for parsing tunnel specifications and port ranges.
package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	ncerr "gonc/internal/errors"
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

	// ── Reverse SSH tunnel ────────────────────────────────────────────
	ReverseTunnelSpec    string // raw user@host[:port] from -R
	ReverseTunnelEnabled bool
	ReverseTunnelUser    string
	ReverseTunnelHost    string
	ReverseTunnelPort    int    // SSH port on gateway
	RemotePort           int    // port to bind on remote gateway
	RemoteBindAddress    string // 0.0.0.0 or specific IP
	CheckGatewayPorts    bool
	KeepAliveInterval    int // seconds (0 = disable)
	AutoReconnect        bool

	// ── Execution ────────────────────────────────────────────────────
	Execute string // -e: program path
	Command string // -c: shell command

	// ── Output ───────────────────────────────────────────────────────
	Verbose int
	ZeroIO  bool

	// ── Diagnostics ──────────────────────────────────────────────────
	DryRun bool // validate config and exit without executing
}

// ── Port helpers ─────────────────────────────────────────────────────

// PortRange is an inclusive start-end pair.
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
		return "", "", 0, fmt.Errorf("invalid tunnel spec %q - expected [user@]host[:port]", spec)
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
// Errors returned are [ncerr.ConfigError] when the field is known.
func (c *Config) Validate() error {
	if c.Listen {
		if c.LocalPort == 0 {
			return &ncerr.ConfigError{
				Field:   "port",
				Message: "required in listen mode",
				Hint:    "specify a port with -p <port>, e.g.: gonc -l -p 8080",
			}
		}
		if c.ZeroIO {
			return &ncerr.ConfigError{
				Field:   "zero-io",
				Message: "listen mode and zero-I/O mode are mutually exclusive",
				Hint:    "use -z without -l for port scanning",
			}
		}
		if c.TunnelEnabled {
			return &ncerr.ConfigError{
				Field:   "tunnel",
				Message: "listen mode through a forward SSH tunnel (-T) is not supported",
				Hint:    "use -R for reverse tunnels instead",
			}
		}
	} else {
		if c.Host == "" && !c.ReverseTunnelEnabled {
			return &ncerr.ConfigError{
				Field:   "host",
				Message: "hostname is required",
				Hint:    "usage: gonc [options] <host> <port>",
			}
		}
		if c.Port == 0 && len(c.Ports) == 0 && !c.ReverseTunnelEnabled {
			return &ncerr.ConfigError{
				Field:   "port",
				Message: "destination port is required",
				Hint:    "usage: gonc <host> <port>, e.g.: gonc example.com 80",
			}
		}
	}

	// ── reverse tunnel validation ───────────────────────────────
	if c.ReverseTunnelEnabled {
		if !c.Listen {
			return &ncerr.ConfigError{
				Field:   "reverse-tunnel",
				Message: "reverse tunnel requires listen mode",
				Hint:    "-R implies -l automatically; this is an internal error",
			}
		}
		if c.RemotePort == 0 {
			return &ncerr.ConfigError{
				Field:   "remote-port",
				Message: "required with -R",
				Hint:    "e.g.: gonc -p 3000 -R serveo.net --remote-port 80",
			}
		}
		if c.RemotePort < 1 || c.RemotePort > 65535 {
			return &ncerr.ConfigError{
				Field:   "remote-port",
				Value:   c.RemotePort,
				Message: "out of range 1-65535",
			}
		}
		if c.ReverseTunnelHost == "" {
			return &ncerr.ConfigError{
				Field:   "reverse-tunnel",
				Message: "tunnel host is required",
				Hint:    "e.g.: gonc -R user@gateway --remote-port 9000 -p 8080",
			}
		}
		if c.TunnelEnabled {
			return &ncerr.ConfigError{
				Field:   "tunnel",
				Message: "-T and -R are mutually exclusive",
				Hint:    "use either forward tunnel (-T) or reverse tunnel (-R)",
			}
		}
		if c.UDP {
			return &ncerr.ConfigError{
				Field:   "udp",
				Message: "reverse tunnel does not support UDP",
			}
		}
	}

	if c.Execute != "" && c.Command != "" {
		return &ncerr.ConfigError{
			Field:   "exec",
			Message: "-e and -c are mutually exclusive",
			Hint:    "use -e for a program or -c for a shell command, not both",
		}
	}

	if c.UDP && c.TunnelEnabled {
		return &ncerr.ConfigError{
			Field:   "udp",
			Message: "UDP is not supported through SSH tunnels",
		}
	}

	if c.TunnelEnabled && c.TunnelHost == "" {
		return &ncerr.ConfigError{
			Field:   "tunnel",
			Message: "tunnel host is required",
			Hint:    "e.g.: gonc -T user@gateway host port",
		}
	}

	return nil
}
