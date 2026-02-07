package tunnel

// reverse_health.go - keepalive, reconnection, and sleep helpers
// for the reverse tunnel.

import (
	"context"
	"fmt"
	"time"
)

// keepaliveLoop sends periodic SSH keep-alive requests and closes the
// listener if the connection has died, letting acceptLoop handle
// reconnection.
func (rt *ReverseTunnel) keepaliveLoop() {
	defer rt.wg.Done()

	ticker := time.NewTicker(rt.config.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rt.ctx.Done():
			return
		case <-ticker.C:
			rt.mu.Lock()
			client := rt.client
			rt.mu.Unlock()

			if client == nil {
				return
			}

			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				rt.logger.Error("SSH keepalive failed: %v", err)
				rt.metrics.RecordError(fmt.Sprintf("keepalive: %v", err))
				// Close the listener to unblock Accept so the
				// acceptLoop can handle reconnection.
				rt.mu.Lock()
				if rt.listener != nil {
					rt.listener.Close()
					rt.listener = nil
				}
				rt.mu.Unlock()
				return
			}
			rt.metrics.RecordHealthCheck()
			rt.logger.Debug("SSH keepalive OK")
		}
	}
}

// reconnect tears down the current tunnel and re-establishes it with
// exponential backoff.  It is only called from acceptLoop.
func (rt *ReverseTunnel) reconnect() error {
	rt.logger.Info("reverse tunnel: reconnecting...")
	rt.metrics.TunnelReconnect()

	// Tear down old resources.
	rt.mu.Lock()
	if rt.listener != nil {
		rt.listener.Close()
		rt.listener = nil
	}
	if rt.client != nil {
		rt.client.Close()
		rt.client = nil
	}
	rt.mu.Unlock()

	backoff := time.Second
	const maxBackoff = 60 * time.Second
	const maxAttempts = 10

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if rt.ctx.Err() != nil {
			return rt.ctx.Err()
		}

		client, err := rt.dialSSH(rt.ctx)
		if err != nil {
			rt.logger.Error("reconnect %d/%d SSH: %v", attempt, maxAttempts, err)
			rt.metrics.RecordError(fmt.Sprintf("reconnect SSH attempt %d: %v", attempt, err))
			sleepCtx(rt.ctx, backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		listener, err := listenRemoteForward(client, rt.config.RemoteBindAddress, rt.config.RemotePort)
		if err != nil {
			rt.logger.Error("reconnect %d/%d listen: %v", attempt, maxAttempts, err)
			rt.metrics.RecordError(fmt.Sprintf("reconnect listen attempt %d: %v", attempt, err))
			client.Close()
			sleepCtx(rt.ctx, backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		rt.mu.Lock()
		rt.client = client
		rt.listener = listener
		rt.mu.Unlock()

		rt.logger.Info("reverse tunnel: reconnected successfully")

		// Restart keepalive with the new client.
		if rt.config.KeepAliveInterval > 0 {
			rt.wg.Add(1)
			go rt.keepaliveLoop()
		}

		return nil
	}

	return fmt.Errorf("failed to reconnect after %d attempts", maxAttempts)
}

// sleepCtx sleeps for at most d, returning early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
