// Package netcat implements the core connect / listen / scan modes.
package netcat

import (
	"context"
	"fmt"

	"gonc/config"
	"gonc/tunnel"
	"gonc/util"
)

// NetCat orchestrates a single session.
type NetCat struct {
	Config *config.Config
	Tunnel tunnel.Tunnel
	Logger *util.Logger
}

// New returns a ready-to-run NetCat.
func New(cfg *config.Config, tun tunnel.Tunnel, logger *util.Logger) *NetCat {
	return &NetCat{Config: cfg, Tunnel: tun, Logger: logger}
}

// Run connects a tunnel (if any) and dispatches to the correct mode.
func (nc *NetCat) Run(ctx context.Context) error {
	if nc.Tunnel != nil {
		nc.Logger.Verbose("establishing SSH tunnel to %s@%s:%d",
			nc.Config.TunnelUser, nc.Config.TunnelHost, nc.Config.TunnelPort)
		if err := nc.Tunnel.Connect(ctx); err != nil {
			return fmt.Errorf("tunnel: %w", err)
		}
		defer nc.Tunnel.Close()
		nc.Logger.Verbose("SSH tunnel established")
	}

	switch {
	case nc.Config.Listen:
		return nc.handleServer(ctx)
	case nc.Config.ZeroIO:
		return nc.handleScan(ctx)
	default:
		return nc.handleClient(ctx)
	}
}
