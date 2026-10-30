package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// testAddr returns an address unique to this test. Unix socket paths are
// short-limited, so the Unix directory is made under the system temp root.
func testAddr(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\mpx-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	}
	dir, err := os.MkdirTemp("", "mpx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	// The "run" subdirectory does not exist yet, so Listen must create it.
	return filepath.Join(dir, "run", "mpx.sock")
}

func TestEchoRoundTrip(t *testing.T) {
	addr := testAddr(t)
	ln, err := Listen(addr)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := Dial(ctx, addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello, daemon")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if string(got) != string(msg) {
		t.Errorf("echo = %q, want %q", got, msg)
	}
}

func TestListenAddrInUse(t *testing.T) {
	addr := testAddr(t)
	first, err := Listen(addr)
	if err != nil {
		t.Fatalf("first Listen: %v", err)
	}
	defer first.Close()

	second, err := Listen(addr)
	if second != nil {
		second.Close()
	}
	if !errors.Is(err, ErrAddrInUse) {
		t.Fatalf("second Listen err = %v, want ErrAddrInUse", err)
	}
}

func TestDialMissing(t *testing.T) {
	addr := testAddr(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := Dial(ctx, addr)
	if err == nil {
		conn.Close()
		t.Fatal("Dial to a missing address succeeded")
	}
}
