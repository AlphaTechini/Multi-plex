package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestHookCommand(t *testing.T) {
	dir := t.TempDir()
	mpx := filepath.Join(dir, "mpx")
	if got := HookCommand(mpx); !slices.Equal(got, []string{mpx, "hook"}) {
		t.Fatalf("without mpx-hook: %v", got)
	}
	name := "mpx-hook"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	hook := filepath.Join(dir, name)
	if err := os.WriteFile(hook, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := HookCommand(mpx); !slices.Equal(got, []string{hook}) {
		t.Fatalf("with mpx-hook: %v", got)
	}
}
