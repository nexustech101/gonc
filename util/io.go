package util

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
)

// DefaultBufSize is the standard buffer size for network I/O (32 KiB).
const DefaultBufSize = 32 * 1024

// BidirectionalCopy shuffles data between a network connection and an
// arbitrary reader/writer pair (typically stdin/stdout) until one side
// reaches EOF or the context is cancelled.
func BidirectionalCopy(ctx context.Context, conn net.Conn, r io.Reader, w io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	// network → writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.Copy(w, conn)
		errCh <- err
		cancel()
	}()

	// reader → network
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.Copy(conn, r)
		// Half-close the write side so the remote knows we're done
		// sending, but keep the read side open to drain any remaining
		// data from the server (the writer goroutine handles that).
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite() //nolint:errcheck
		}
		errCh <- err
		// Only cancel on real errors; a normal EOF from the reader
		// should NOT tear down the connection before the remote
		// finishes sending.
		if err != nil {
			cancel()
		}
	}()

	<-ctx.Done()
	conn.Close() // unblock any pending reads/writes
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && !isHarmless(err) {
			return err
		}
	}
	return nil
}

// isHarmless returns true for errors that are expected during shutdown.
func isHarmless(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	// net.OpError wrapping "use of closed network connection"
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return errors.Is(opErr.Err, net.ErrClosed)
	}
	return false
}
