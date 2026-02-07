package tunnel

// reverse_forwarder.go - connection bridging for the reverse tunnel.

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// handleConnection bridges a single remote connection to the local service.
func (rt *ReverseTunnel) handleConnection(remoteConn net.Conn) {
	defer rt.wg.Done()
	defer remoteConn.Close()
	defer rt.metrics.ConnectionClosed()

	start := time.Now()
	remoteAddr := remoteConn.RemoteAddr().String()

	localTarget := net.JoinHostPort(rt.config.LocalAddress, fmt.Sprintf("%d", rt.config.LocalPort))
	localConn, err := net.DialTimeout("tcp", localTarget, 5*time.Second)
	if err != nil {
		rt.logger.Error("reverse tunnel: local dial %s failed: %v",
			localTarget, err)
		rt.metrics.RecordError(fmt.Sprintf("local dial %s: %v", localTarget, err))
		return
	}
	defer localConn.Close()

	rt.logger.Info("reverse tunnel: bridging %s ↔ %s", remoteAddr, localTarget)

	in, out := bridgeConns(rt.ctx, remoteConn, localConn)
	rt.metrics.BytesReceived(in)
	rt.metrics.BytesSent(out)

	rt.logger.Info("reverse tunnel: %s closed after %v (in=%d out=%d)",
		remoteAddr, time.Since(start).Truncate(time.Millisecond), in, out)
}

// bridgeConns copies data bidirectionally between two connections
// until one side closes or the context is cancelled.  It returns the
// number of bytes transferred in each direction.
func bridgeConns(ctx context.Context, a, b net.Conn) (aToB, bToA int64) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		n, _ := io.Copy(b, a)
		aToB = n
		cancel()
	}()

	go func() {
		defer wg.Done()
		n, _ := io.Copy(a, b)
		bToA = n
		cancel()
	}()

	<-ctx.Done()
	a.Close()
	b.Close()
	wg.Wait()
	return aToB, bToA
}
