package polkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/core"
)

// TestMain doubles as a fake pkexec: MPX_FAKE_PKEXEC="<exit>:<stderr>".
func TestMain(m *testing.M) {
	if mode := os.Getenv("MPX_FAKE_PKEXEC"); mode != "" {
		var code int
		var msg string
		fmt.Sscanf(mode, "%d:%s", &code, &msg)
		fmt.Fprint(os.Stderr, strings.ReplaceAll(msg, "-", " "))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func fakeVerifier(t *testing.T, mode string) *Verifier {
	t.Helper()
	t.Setenv("MPX_FAKE_PKEXEC", mode)
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Verifier{pkexec: exe, trueBin: "/bin/true", getenv: os.Getenv}
}

func TestVerifyMapping(t *testing.T) {
	tests := []struct {
		mode string
		want error
	}{
		{"1:x", approval.ErrFailed}, // pkexec ran the program, which failed: still not approved
		{"126:", approval.ErrCancelled},
		{"127:Not-authorized", approval.ErrFailed},
		{"127:No-authentication-agent-found", approval.ErrUnavailable},
		{"127:Error-getting-authority", approval.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			err := fakeVerifier(t, tt.mode).Verify(context.Background(), core.ApprovalRequest{}, "m")
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestVerifySuccess(t *testing.T) {
	// Exit 0 cannot go through "%d:%s" with an empty mode, so use "0:ok".
	if err := fakeVerifier(t, "0:ok").Verify(context.Background(), core.ApprovalRequest{}, "m"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyMissingPkexec(t *testing.T) {
	v := &Verifier{pkexec: filepath.Join(t.TempDir(), "nope"), trueBin: "/bin/true", getenv: os.Getenv}
	if err := v.Verify(context.Background(), core.ApprovalRequest{}, "m"); !errors.Is(err, approval.ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestAvailable(t *testing.T) {
	proc := t.TempDir()
	mk := func(pid, comm string) {
		d := filepath.Join(proc, pid)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "comm"), []byte(comm+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("100", "bash")
	env := map[string]string{}
	v := &Verifier{pkexec: "/usr/bin/pkexec", trueBin: "/bin/true", procDir: proc,
		getenv: func(k string) string { return env[k] }}
	ctx := context.Background()

	if v.Available(ctx) {
		t.Fatal("no display: must be unavailable")
	}
	env["DISPLAY"] = ":0"
	if v.Available(ctx) {
		t.Fatal("no agent process: must be unavailable")
	}
	mk("200", "polkit-gnome-au")
	if !v.Available(ctx) {
		t.Fatal("display + agent: want available")
	}
	delete(env, "DISPLAY")
	env["WAYLAND_DISPLAY"] = "wayland-0"
	if !v.Available(ctx) {
		t.Fatal("wayland + agent: want available")
	}
}

func TestOnlyLinux(t *testing.T) {
	v := New()
	if runtime.GOOS != "linux" && v.Available(context.Background()) {
		t.Fatal("must be unavailable off Linux")
	}
	if v.Name() != "polkit" {
		t.Fatal("name")
	}
}
