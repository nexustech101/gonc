package netcat

import (
	"context"
	"fmt"
	"time"

	"gonc/internal/metrics"
	"gonc/tunnel"
)

// handleReverseTunnel establishes a reverse SSH tunnel that exposes
// the local service (Config.LocalPort) on the remote gateway
// (Config.RemotePort) and blocks until the context is cancelled or the
// tunnel shuts down.
func (nc *NetCat) handleReverseTunnel(ctx context.Context) error {
	sshCfg := &tunnel.SSHConfig{
		User:              nc.Config.ReverseTunnelUser,
		Host:              nc.Config.ReverseTunnelHost,
		Port:              nc.Config.ReverseTunnelPort,
		KeyPath:           nc.Config.SSHKeyPath,
		PromptPass:        nc.Config.SSHPassword,
		UseAgent:          nc.Config.UseSSHAgent,
		StrictHostKey:     nc.Config.StrictHostKey,
		KnownHosts:        nc.Config.KnownHostsPath,
		AllowKeyboardInteractive: true, // enables auth for public tunnel services like serveo.net
	}

	var keepAlive time.Duration
	if nc.Config.KeepAliveInterval > 0 {
		keepAlive = time.Duration(nc.Config.KeepAliveInterval) * time.Second
	}

	rtCfg := &tunnel.ReverseTunnelConfig{
		SSHConfig:         sshCfg,
		RemoteBindAddress: nc.Config.RemoteBindAddress,
		RemotePort:        nc.Config.RemotePort,
		LocalAddress:      "127.0.0.1",
		LocalPort:         nc.Config.LocalPort,
		CheckGatewayPorts: nc.Config.CheckGatewayPorts,
		KeepAliveInterval: keepAlive,
		AutoReconnect:     nc.Config.AutoReconnect,
	}

	nc.Logger.Verbose("establishing reverse tunnel: "+
		"%s@%s:%d remote-port=%d → local=%d",
		sshCfg.User, sshCfg.Host, sshCfg.Port,
		rtCfg.RemotePort, rtCfg.LocalPort)

	rt := tunnel.NewReverseTunnel(rtCfg, nc.Logger, metrics.New())

	if err := rt.Start(ctx); err != nil {
		return fmt.Errorf("reverse tunnel: %w", err)
	}
	defer rt.Close()

	// Block until the context is cancelled or the tunnel shuts down.
	rt.Wait()
	return nil
}
