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

	// ── output ───────────────────────────────────────────────────
	fs.CountVarP(&cfg.Verbose, "verbose", "v", "Increase verbosity (repeatable)")

	var showVersion, showHelp bool
	fs.BoolVar(&showVersion, "version", false, "Print version and exit")
	fs.BoolVarP(&showHelp, "help", "h", false, "Show this help")

	fs.Usage = func() { printUsage(fs) }

	// ── parse ────────────────────────────────────────────────────
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
	}

	// ── validate ─────────────────────────────────────────────────
	if err := cfg.Validate(); err != nil {
		return err
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
	fmt.Fprintf(os.Stderr, `GoNC – Network Connectivity Tool v%s

A cross-platform netcat implementation with SSH tunneling.

Usage:
  gonc [options] <host> <port> [ports...]     Connect
  gonc -l -p <port> [options]                 Listen
  gonc -z [options] <host> <ports...>         Scan
  gonc -T user@gateway <host> <port>          Tunnel

Options:
`, version)
	fs.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Examples:
  gonc example.com 80                         TCP connect
  gonc -l -p 8080                             Listen on 8080
  gonc -vz host.example.com 20-25 80 443      Port scan
  gonc -T admin@bastion db-internal 5432      SSH tunnel
  echo "hello" | gonc host.example.com 9000   Pipe data
`)
}
