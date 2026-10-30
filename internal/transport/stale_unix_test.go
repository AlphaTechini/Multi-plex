//go:build !windows

package transport

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestStaleSocketRemoved(t *testing.T) {
	addr := testAddr(t)
	ln, err := Listen(addr)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	// Keep the socket file after close, as a crashed daemon would.
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	if _, err := os.Lstat(addr); err != nil {
		t.Fatalf("expected stale socket file to remain: %v", err)
	}

	again, err := Listen(addr)
	if err != nil {
		t.Fatalf("Listen over stale socket: %v", err)
	}
	defer again.Close()

	fi, err := os.Stat(addr)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("socket mode = %v, want 0600", perm)
	}
	di, err := os.Stat(filepath.Dir(addr))
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("parent dir mode = %v, want 0700", perm)
	}
}

func TestNonSocketFileRejected(t *testing.T) {
	addr := testAddr(t)
	if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(addr, []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(addr); err == nil {
		t.Fatal("Listen over a regular file succeeded")
	}
	if _, err := os.Stat(addr); err != nil {
		t.Errorf("regular file was removed: %v", err)
	}
}

func TestDialTimeoutHonoursContext(t *testing.T) {
	addr := testAddr(t)
	ln, err := Listen(addr)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := Dial(ctx, addr)
	if conn != nil {
		conn.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Dial with cancelled ctx err = %v, want context.Canceled", err)
	}
}
