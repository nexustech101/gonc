package core

import (
	"fmt"
	"net"
	"time"

	"gonc/config"
	"gonc/internal/capability"
	"gonc/internal/transport"
	"gonc/tunnel"
	"gonc/util"
)

// Build constructs the appropriate Mode from the given configuration.
// This is the single dispatch point that replaces the scattered
// switch/if trees in the old architecture.
func Build(cfg *config.Config, logger *util.Logger) (Mode, error) {
	switch {
	case cfg.ReverseTunnelEnabled:
		return buildReverseTunnel(cfg, logger)
	case cfg.Listen:
		return buildListen(cfg, logger)
	case cfg.ZeroIO:
		return buildScan(cfg, logger)
	default:
		return buildConnect(cfg, logger)
	}
}

// ── mode builders ────────────────────────────────────────────────────

func buildConnect(cfg *config.Config, logger *util.Logger) (Mode, error) {
	if cfg.NoDNS && net.ParseIP(cfg.Host) == nil {
		return nil, fmt.Errorf(
			"cannot parse %q as an IP address (DNS disabled with -n)",
			cfg.Host)
	}

	address := util.FormatAddr(cfg.Host, cfg.Port)
	network := "tcp"
	if cfg.UDP {
		network = "udp"
	}

	return &ConnectMode{
		Dialer:     buildDialer(cfg, logger),
		Capability: buildCapability(cfg),
		Network:    network,
		Address:    address,
		Logger:     logger,
	}, nil
}

func buildListen(cfg *config.Config, logger *util.Logger) (Mode, error) {
	address := fmt.Sprintf(":%d", cfg.LocalPort)
	network := "tcp"
	if cfg.UDP {
		network = "udp"
	}

	return &ListenMode{
		Address:    address,
		Network:    network,
		KeepOpen:   cfg.KeepOpen,
		Timeout:    cfg.Timeout,
		Capability: buildCapability(cfg),
		Logger:     logger,
	}, nil
}

func buildScan(cfg *config.Config, logger *util.Logger) (Mode, error) {
	if cfg.NoDNS && net.ParseIP(cfg.Host) == nil {
		return nil, fmt.Errorf(
			"cannot parse %q as an IP address (DNS disabled with -n)",
			cfg.Host)
	}

	ports := cfg.AllPorts()
	if len(ports) == 0 && cfg.Port > 0 {
		ports = []int{cfg.Port}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = config.DefaultScanTimeout
	}

	return &ScanMode{
		Dialer:  buildDialer(cfg, logger),
		Host:    cfg.Host,
		Ports:   ports,
		Timeout: timeout,
		Logger:  logger,
		Verbose: cfg.Verbose,
	}, nil
}

func buildReverseTunnel(cfg *config.Config, logger *util.Logger) (Mode, error) {
	sshCfg := &tunnel.SSHConfig{
		User:                     cfg.ReverseTunnelUser,
		Host:                     cfg.ReverseTunnelHost,
		Port:                     cfg.ReverseTunnelPort,
		KeyPath:                  cfg.SSHKeyPath,
		PromptPass:               cfg.SSHPassword,
		UseAgent:                 cfg.UseSSHAgent,
		StrictHostKey:            cfg.StrictHostKey,
		KnownHosts:               cfg.KnownHostsPath,
		AllowKeyboardInteractive: true,
	}

	var keepAlive time.Duration
	if cfg.KeepAliveInterval > 0 {
		keepAlive = time.Duration(cfg.KeepAliveInterval) * time.Second
	}

	return &ReverseTunnelMode{
		SSHConfig:         sshCfg,
		RemoteBindAddress: cfg.RemoteBindAddress,
		RemotePort:        cfg.RemotePort,
		LocalAddress:      config.DefaultLocalAddress,
		LocalPort:         cfg.LocalPort,
		CheckGatewayPorts: cfg.CheckGatewayPorts,
		KeepAliveInterval: keepAlive,
		AutoReconnect:     cfg.AutoReconnect,
		Logger:            logger,
	}, nil
}

// ── shared helpers ───────────────────────────────────────────────────

// buildDialer creates the right transport.Dialer for the given config.
func buildDialer(cfg *config.Config, logger *util.Logger) transport.Dialer {
	if cfg.TunnelEnabled {
		return transport.NewSSHDialer(&tunnel.SSHConfig{
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

	if cfg.UDP {
		return &transport.UDPDialer{
			Timeout:   cfg.Timeout,
			LocalPort: localPortForConnect(cfg),
		}
	}

	return &transport.TCPDialer{
		Timeout:   cfg.Timeout,
		LocalPort: localPortForConnect(cfg),
	}
}

// buildCapability selects the per-connection behaviour.
func buildCapability(cfg *config.Config) capability.Capability {
	if cfg.Execute != "" || cfg.Command != "" {
		return &capability.Exec{
			Program: cfg.Execute,
			Command: cfg.Command,
		}
	}
	return &capability.Relay{}
}

// localPortForConnect returns the source-port binding for connect mode,
// or 0 if in listen mode (where LocalPort is the listen port).
func localPortForConnect(cfg *config.Config) int {
	if cfg.Listen {
		return 0
	}
	return cfg.LocalPort
}
