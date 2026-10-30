package daemon

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// Serve accepts connections on l until ctx is cancelled or Shutdown is
// called, then shuts the daemon down: it stops accepting, closes client
// connections, kills the panes, flushes and closes the store, and returns.
func (d *Daemon) Serve(ctx context.Context, l net.Listener) error {
	d.setAddr(l)
	d.startBackground()
	d.log.Info("daemon serving", "addr", l.Addr().String(), "version", d.version)

	go func() {
		select {
		case <-ctx.Done():
			d.Shutdown()
		case <-d.stopCh:
		}
	}()
	go func() {
		<-d.stopCh
		_ = l.Close() // unblocks Accept
	}()

	var acceptErr error
	for {
		nc, err := l.Accept()
		if err != nil {
			select {
			case <-d.stopCh:
			default:
				acceptErr = err
				d.Shutdown()
			}
			break
		}
		go d.serveConn(nc)
	}
	d.shutdownAll()
	if acceptErr != nil && !errors.Is(acceptErr, net.ErrClosed) {
		return acceptErr
	}
	return nil
}

// Shutdown asks the daemon to stop. It returns immediately; Serve returns
// once the shutdown has finished. Safe to call more than once.
func (d *Daemon) Shutdown() { d.stopOnce.Do(func() { close(d.stopCh) }) }

// Done is closed after Serve has finished shutting down.
func (d *Daemon) Done() <-chan struct{} { return d.doneCh }

// shutdownAll tears everything down in order.
func (d *Daemon) shutdownAll() {
	defer close(d.doneCh)

	// Stop background work and refuse new panes.
	d.cancel()

	// Close client connections.
	d.mu.RLock()
	conns := make([]*conn, 0, len(d.conns))
	for c := range d.conns {
		conns = append(conns, c)
	}
	d.mu.RUnlock()
	for _, c := range conns {
		c.cancel()
		_ = c.nc.Close()
	}

	// Wait for the connections' request goroutines to finish: they end once
	// their contexts are cancelled, and none may start background work after.
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		d.mu.RLock()
		left := len(d.conns)
		d.mu.RUnlock()
		if left == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Kill the panes, in parallel: each Close waits for its process.
	d.mu.RLock()
	panes := make([]*pane, 0, len(d.panes))
	for _, p := range d.panes {
		panes = append(panes, p)
	}
	d.mu.RUnlock()
	var wg sync.WaitGroup
	for _, p := range panes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.shutdown()
		}()
	}
	wg.Wait()

	done := make(chan struct{})
	go func() { d.bg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		d.log.Warn("background tasks did not stop in time")
	}

	if err := d.store.Close(); err != nil {
		d.log.Error("close store", "err", err)
	}
	d.log.Info("daemon stopped")
	_ = d.logClose.Close()
}
