package windowshello

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/core"
)

// TestMain doubles as the fake helper: when re-executed with MPX_FAKE_HELLO
// set, the test binary behaves like mpx-hello.exe.
func TestMain(m *testing.M) {
	if mode := os.Getenv("MPX_FAKE_HELLO"); mode != "" {
		fakeHelper(mode)
		return
	}
	os.Exit(m.Run())
}

// fakeHelper mode is "<result>:<detail>:<exit>"; args are echoed to MPX_FAKE_HELLO_ARGS file.
func fakeHelper(mode string) {
	if f := os.Getenv("MPX_FAKE_HELLO_ARGS"); f != "" {
		_ = os.WriteFile(f, []byte(strings.Join(os.Args[1:], "\x00")), 0o600)
	}
	if mode == "hang" {
		time.Sleep(time.Minute)
	}
	if mode == "garbage" {
		fmt.Println("not json")
		os.Exit(0)
	}
	p := strings.SplitN(mode, ":", 3)
	var exit int
	fmt.Sscan(p[2], &exit)
	fmt.Printf("{\"result\":%q,\"detail\":%q}\n", p[0], p[1])
	os.Exit(exit)
}

// fake returns a verifier whose helper is this test binary in the given mode.
func fake(t *testing.T, mode string) *Verifier {
	t.Helper()
	t.Setenv("MPX_FAKE_HELLO", mode)
	t.Setenv("GORACE", "atexit_sleep_ms=0") // the race runtime otherwise sleeps 1s at exit
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	v := New(exe)
	v.onlyWindows = false
	return v
}

func TestVerifyMapping(t *testing.T) {
	tests := []struct {
		mode string
		want error // nil means success; errors.Is match otherwise
	}{
		{"verified::0", nil},
		{"canceled:the user canceled:1", approval.ErrCancelled},
		{"not_available:x:2", approval.ErrUnavailable},
		{"not_configured:x:2", approval.ErrUnavailable},
		{"disabled_by_policy:x:2", approval.ErrUnavailable},
		{"device_busy:x:2", approval.ErrUnavailable},
		{"retries_exhausted:x:3", approval.ErrFailed},
		{"error:boom:3", approval.ErrFailed},
		{"error:timeout:4", approval.ErrTimeout},
		{"weird::0", approval.ErrFailed},
		{"verified::3", approval.ErrFailed}, // result and exit code disagree: fail closed
		{"garbage", approval.ErrFailed},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			v := fake(t, tt.mode)
			err := v.Verify(context.Background(), core.ApprovalRequest{}, "msg")
			if tt.want == nil && err != nil {
				t.Fatalf("got %v, want nil", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestVerifyArgs(t *testing.T) {
	long := strings.Repeat("é", 500)
	got := verifyArgs(long, 90*time.Second)
	if len(got) != 5 || got[0] != "verify" || got[1] != "--message" || got[3] != "--timeout" || got[4] != "90" {
		t.Fatalf("args = %q", got)
	}
	if n := len([]rune(got[2])); n != maxMessageRunes {
		t.Fatalf("message runes = %d", n)
	}
	if g := verifyArgs("hi", 0); g[4] != "1" {
		t.Fatalf("timeout floor: %q", g)
	}
}

func TestVerifyPassesArgsWithoutShell(t *testing.T) {
	argsFile := t.TempDir() + "/args"
	t.Setenv("MPX_FAKE_HELLO_ARGS", argsFile)
	v := fake(t, "verified::0")
	msg := `git push --force "origin" ; rm -rf $HOME`
	if err := v.Verify(context.Background(), core.ApprovalRequest{}, msg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(string(b), "\x00")
	if len(got) != 5 || got[2] != msg {
		t.Fatalf("args = %q", got)
	}
}

func TestVerifyContextCancel(t *testing.T) {
	v := fake(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := v.Verify(ctx, core.ApprovalRequest{}, "m"); !errors.Is(err, approval.ErrTimeout) {
		t.Fatalf("got %v", err)
	}
}

func TestAvailable(t *testing.T) {
	if v := New(""); v.Available(context.Background()) {
		t.Fatal("empty path must be unavailable")
	}
	if v := fake(t, "available::0"); !v.Available(context.Background()) {
		t.Fatal("expected available")
	}
	if v := fake(t, "not_configured::2"); v.Available(context.Background()) {
		t.Fatal("expected unavailable")
	}
	// On non-Windows the real constructor is never available.
	exe, _ := os.Executable()
	if v := New(exe); v.Available(context.Background()) && !isWindows() {
		t.Fatal("must be unavailable off Windows")
	}
}

func TestAvailableCached(t *testing.T) {
	v := fake(t, "available::0")
	if !v.Available(context.Background()) {
		t.Fatal("expected available")
	}
	t.Setenv("MPX_FAKE_HELLO", "not_available::2") // would flip the answer if re-run
	if !v.Available(context.Background()) {
		t.Fatal("expected cached answer")
	}
	v.mu.Lock()
	v.checked = time.Now().Add(-2 * availabilityTTL)
	v.mu.Unlock()
	if v.Available(context.Background()) {
		t.Fatal("expected refreshed answer")
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/h"
	if err := os.WriteFile(p, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPX_HELLO_PATH", p)
	if got := Find(); got != p {
		t.Fatalf("Find = %q", got)
	}
	t.Setenv("MPX_HELLO_PATH", dir+"/missing")
	if got := Find(); got != "" {
		t.Fatalf("Find = %q, want empty", got)
	}
}

func TestMissingHelper(t *testing.T) {
	v := New("/nonexistent/mpx-hello")
	err := v.Verify(context.Background(), core.ApprovalRequest{}, "m")
	if !errors.Is(err, approval.ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func isWindows() bool {
	_, err := exec.LookPath("cmd.exe")
	return err == nil && os.PathSeparator == '\\'
}
