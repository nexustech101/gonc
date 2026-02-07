// Package cmd wires up the CLI flags and dispatches to the netcat core.
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"gonc/config"
	"gonc/netcat"
	"gonc/tunnel"
	"gonc/util"
)

// version is overridable at link time:
//
//	go build -ldflags "-X gonc/cmd.version=2.0.0"
var version = "1.0.0" //nolint:gochecknoglobals

// Execute parses args and runs the appropriate gonc mode.
func Execute(ctx context.Context, args []string) error {
	cfg := &config.Config{}
	fs := flag.NewFlagSet("gonc", flag.ContinueOnError)

	// ── connection ───────────────────────────────────────────────
	fs.BoolVarP(&cfg.Listen, "listen", "l", false, "Listen mode")
	fs.IntVarP(&cfg.LocalPort, "port", "p", 0, "Local port number")
	fs.BoolVarP(&cfg.UDP, "udp", "u", false, "UDP mode")
	fs.BoolVarP(&cfg.NoDNS, "no-dns", "n", false, "Numeric-only, no DNS resolution")
	fs.BoolVarP(&cfg.KeepOpen, "keep-open", "k", false, "Accept multiple connections (with -l)")
	fs.BoolVarP(&cfg.ZeroIO, "zero-io", "z", false, "Zero-I/O mode (port scanning)")

	var timeoutSec int
	fs.IntVarP(&timeoutSec, "timeout", "w", 0, "Timeout in seconds")

	// ── execution ────────────────────────────────────────────────
	fs.StringVarP(&cfg.Execute, "exec", "e", "", "Execute program after connect")
	fs.StringVarP(&cfg.Command, "command", "c", "", "Execute shell command after connect")

	// ── SSH tunnel ───────────────────────────────────────────────
	fs.StringVarP(&cfg.TunnelSpec, "tunnel", "T", "", "SSH tunnel via [user@]host[:port]")
	fs.StringVar(&cfg.SSHKeyPath, "ssh-key", "", "SSH private key file")
	fs.BoolVar(&cfg.SSHPassword, "ssh-password", false, "Prompt for SSH password")
	fs.BoolVar(&cfg.UseSSHAgent, "ssh-agent", false, "Use SSH agent")
	fs.BoolVar(&cfg.StrictHostKey, "strict-hostkey", false, "Verify SSH host keys")
	fs.StringVar(&cfg.KnownHostsPath, "known-hosts", "", "Custom known_hosts path")
	fs.IntVar(&cfg.TunnelLocalPort, "tunnel-local-port", 0, "Local tunnel port (auto if 0)")

	// ── Reverse SSH tunnel ──────────────────────────────────────
	fs.StringVarP(&cfg.ReverseTunnelSpec, "reverse-tunnel", "R", "", "Reverse SSH tunnel via [user@]host[:port]")
	fs.IntVar(&cfg.RemotePort, "remote-port", 0, "Port to bind on remote gateway (for -R)")
	fs.StringVar(&cfg.RemoteBindAddress, "remote-bind-address", "", "Remote bind address (for -R)")
	fs.BoolVar(&cfg.CheckGatewayPorts, "gateway-ports-check", false, "Verify GatewayPorts before tunneling")
	fs.IntVar(&cfg.KeepAliveInterval, "keep-alive", 30, "SSH keepalive interval in seconds (0 to disable)")
	fs.BoolVar(&cfg.AutoReconnect, "auto-reconnect", false, "Auto-reconnect on tunnel drop")

	// ── output / diagnostics ─────────────────────────────────────
	fs.CountVarP(&cfg.Verbose, "verbose", "v", "Increase verbosity (repeatable)")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Validate config and exit without executing")

	var showVersion, showHelp bool
	fs.BoolVar(&showVersion, "version", false, "Print version and exit")
	fs.BoolVarP(&showHelp, "help", "h", false, "Show this help")

	fs.Usage = func() { printUsage(fs) }

	// ── load environment variables (before flag parsing) ─────────
	config.LoadFromEnv(cfg)

	// ── parse CLI flags (overrides env vars) ─────────────────────
	if err := fs.Parse(args); err != nil {
		return err
	}

	if showHelp || len(args) == 0 {
		printUsage(fs)
		return nil
	}
	if showVersion {
		fmt.Printf("gonc %s\n", version)
		return nil
	}

	if timeoutSec > 0 {
		cfg.Timeout = time.Duration(timeoutSec) * time.Second
	}

	// ── reverse tunnel spec (before positional parsing so that ────
	// ── -R can imply listen mode and skip hostname requirement) ───
	if cfg.ReverseTunnelSpec != "" {
		user, host, port, err := config.ParseTunnelSpec(cfg.ReverseTunnelSpec)
		if err != nil {
			return fmt.Errorf("reverse tunnel: %w", err)
		}
		cfg.ReverseTunnelEnabled = true
		cfg.ReverseTunnelUser = user
		cfg.ReverseTunnelHost = host
		cfg.ReverseTunnelPort = port

		// Default to OS username when no user@ prefix is given,
		// matching the behaviour of the ssh command.
		if cfg.ReverseTunnelUser == "" {
			cfg.ReverseTunnelUser = tunnel.DefaultUsername()
		}

		// -R implies listen mode so the user doesn't need to pass -l.
		cfg.Listen = true

		// Default local port to remote port when not explicitly set.
		if cfg.LocalPort == 0 && cfg.RemotePort > 0 {
			cfg.LocalPort = cfg.RemotePort
		}
	}

	// ── positional arguments ─────────────────────────────────────
	if err := parsePositional(cfg, fs.Args()); err != nil {
		return err
	}

	// ── tunnel spec ──────────────────────────────────────────────
	if cfg.TunnelSpec != "" {
		user, host, port, err := config.ParseTunnelSpec(cfg.TunnelSpec)
		if err != nil {
			return fmt.Errorf("tunnel: %w", err)
		}
		cfg.TunnelEnabled = true
		cfg.TunnelUser = user
		cfg.TunnelHost = host
		cfg.TunnelPort = port

		if cfg.TunnelUser == "" {
			cfg.TunnelUser = tunnel.DefaultUsername()
		}
	}

	// ── validate ─────────────────────────────────────────────────
	if err := cfg.Validate(); err != nil {
		return err
	}

	if cfg.DryRun {
		fmt.Fprintln(os.Stderr, "gonc: configuration valid (dry-run)")
		return nil
	}

	// ── build components ─────────────────────────────────────────
	logger := util.NewLogger(cfg.Verbose)

	var tun tunnel.Tunnel
	if cfg.TunnelEnabled {
		tun = tunnel.NewSSHTunnel(&tunnel.SSHConfig{
			User:          cfg.TunnelUser,
			Host:          cfg.TunnelHost,
			Port:          cfg.TunnelPort,
			KeyPath:       cfg.SSHKeyPath,
			PromptPass:    cfg.SSHPassword,
			UseAgent:      cfg.UseSSHAgent,
			StrictHostKey: cfg.StrictHostKey,
			KnownHosts:    cfg.KnownHostsPath,
		}, logger)
	}

	nc := netcat.New(cfg, tun, logger)
	return nc.Run(ctx)
}

// ── helpers ──────────────────────────────────────────────────────────

func parsePositional(cfg *config.Config, remaining []string) error {
	if cfg.Listen {
		switch len(remaining) {
		case 0: // gonc -l -p PORT
		case 1:
			cfg.Host = remaining[0]
		case 2:
			cfg.Host = remaining[0]
			pr, err := config.ParsePortSpec(remaining[1])
			if err != nil {
				return fmt.Errorf("port: %w", err)
			}
			cfg.Port = pr.Start
		default:
			return fmt.Errorf("too many arguments for listen mode")
		}
		return nil
	}

	// Connect / scan mode: host port [port …]
	if len(remaining) < 1 {
		return fmt.Errorf("hostname required (use --help for usage)")
	}
	cfg.Host = remaining[0]

	if len(remaining) < 2 {
		return fmt.Errorf("port required")
	}

	for _, arg := range remaining[1:] {
		pr, err := config.ParsePortSpec(arg)
		if err != nil {
			return fmt.Errorf("port %q: %w", arg, err)
		}
		cfg.Ports = append(cfg.Ports, pr)
	}
	if len(cfg.Ports) > 0 {
		cfg.Port = cfg.Ports[0].Start
	}
	return nil
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, `GoNC - Network Connectivity Tool v%s

A cross-platform netcat implementation with SSH tunneling.
Single static binary. Zero configuration files required.

Usage:
  gonc [options] <host> <port> [ports...]             Connect
  gonc -l -p <port> [options]                         Listen
  gonc -z [options] <host> <ports...>                 Scan
  gonc -T user@gateway <host> <port>                  SSH tunnel (forward)
  gonc -p <port> -R [user@]host --remote-port <port>  Reverse tunnel

Options:
`, version)
	fs.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Environment Variables:
  GONC_HOST, GONC_PORT, GONC_LISTEN, GONC_UDP, GONC_VERBOSE
  GONC_TUNNEL, GONC_SSH_KEY, GONC_SSH_AGENT, GONC_STRICT_HOSTKEY
  GONC_REVERSE_TUNNEL, GONC_REMOTE_PORT, GONC_AUTO_RECONNECT

  Precedence: CLI flags > Environment > Defaults

Examples:
  gonc example.com 80                         TCP connect
  gonc -l -p 8080                             Listen on 8080
  gonc -vz host.example.com 20-25 80 443      Port scan
  gonc -T admin@bastion db-internal 5432      SSH forward tunnel
  echo "hello" | gonc host.example.com 9000   Pipe data

  # Reverse tunnel - expose local port 8080 on gateway port 9000
  gonc -p 8080 -R user@gateway --remote-port 9000

  # Expose local port 3000 via serveo.net (developer tunnel)
  gonc -p 3000 -R serveo.net --remote-port 80

  # Validate configuration without executing
  gonc --dry-run -p 3000 -R serveo.net --remote-port 80
`)
}
