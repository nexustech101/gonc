package core

import (
	"context"
	"fmt"
	"time"

	"gonc/internal/metrics"
	"gonc/tunnel"
	"gonc/util"
)

// ReverseTunnelMode exposes a local service on a remote SSH gateway.
// This is the Go equivalent of ssh -R.
type ReverseTunnelMode struct {
	SSHConfig         *tunnel.SSHConfig
	RemoteBindAddress string
	RemotePort        int
	LocalAddress      string
	LocalPort         int
	CheckGatewayPorts bool
	KeepAliveInterval time.Duration
	AutoReconnect     bool
	Logger            *util.Logger
}

// Run connects to the SSH gateway, requests a remote listener, and
// blocks until the context is cancelled or the tunnel shuts down.
func (m *ReverseTunnelMode) Run(ctx context.Context) error {
	rtCfg := &tunnel.ReverseTunnelConfig{
		SSHConfig:         m.SSHConfig,
		RemoteBindAddress: m.RemoteBindAddress,
		RemotePort:        m.RemotePort,
		LocalAddress:      m.LocalAddress,
		LocalPort:         m.LocalPort,
		CheckGatewayPorts: m.CheckGatewayPorts,
		KeepAliveInterval: m.KeepAliveInterval,
		AutoReconnect:     m.AutoReconnect,
	}

	m.Logger.Verbose("establishing reverse tunnel: "+
		"%s@%s:%d remote-port=%d → local=%d",
		m.SSHConfig.User, m.SSHConfig.Host, m.SSHConfig.Port,
		m.RemotePort, m.LocalPort)

	rt := tunnel.NewReverseTunnel(rtCfg, m.Logger, metrics.New())

	if err := rt.Start(ctx); err != nil {
		return fmt.Errorf("reverse tunnel: %w", err)
	}
	defer rt.Close()

	// Block until the context is cancelled or the tunnel shuts down.
	rt.Wait()
	return nil
}
