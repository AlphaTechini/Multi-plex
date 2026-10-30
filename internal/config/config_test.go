package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultPathsMPXHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvHome, home)

	p, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths: %v", err)
	}
	want := map[string]string{
		"DataDir":      filepath.Join(home, "data"),
		"ConfigDir":    filepath.Join(home, "config"),
		"RuntimeDir":   filepath.Join(home, "run"),
		"DBPath":       filepath.Join(home, "data", "multiplex.db"),
		"WorktreesDir": filepath.Join(home, "data", "worktrees"),
		"LogDir":       filepath.Join(home, "data", "logs"),
	}
	got := map[string]string{
		"DataDir":      p.DataDir,
		"ConfigDir":    p.ConfigDir,
		"RuntimeDir":   p.RuntimeDir,
		"DBPath":       p.DBPath,
		"WorktreesDir": p.WorktreesDir,
		"LogDir":       p.LogDir,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
}

func TestDefaultPathsPlatform(t *testing.T) {
	t.Setenv(EnvHome, "")
	if runtime.GOOS == "linux" {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		t.Setenv("XDG_RUNTIME_DIR", "")
		p, err := DefaultPaths()
		if err != nil {
			t.Fatalf("DefaultPaths: %v", err)
		}
		if want := filepath.Join(data, "multiplex"); p.DataDir != want {
			t.Errorf("DataDir = %q, want %q", p.DataDir, want)
		}
		if !strings.HasPrefix(p.RuntimeDir, os.TempDir()) {
			t.Errorf("RuntimeDir = %q, want under temp dir", p.RuntimeDir)
		}
	}
	p, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths: %v", err)
	}
	if filepath.Base(p.DBPath) != "multiplex.db" || filepath.Dir(p.DBPath) != p.DataDir {
		t.Errorf("DBPath = %q, want multiplex.db inside DataDir", p.DBPath)
	}
	if filepath.Base(p.WorktreesDir) != "worktrees" || filepath.Base(p.LogDir) != "logs" {
		t.Errorf("WorktreesDir/LogDir = %q/%q", p.WorktreesDir, p.LogDir)
	}
	if !strings.HasSuffix(p.ConfigDir, "multiplex") {
		t.Errorf("ConfigDir = %q, want suffix multiplex", p.ConfigDir)
	}
}

func TestEnsure(t *testing.T) {
	p := pathsUnder(t.TempDir())
	for i := 0; i < 2; i++ { // second call must be a no-op
		if err := p.Ensure(); err != nil {
			t.Fatalf("Ensure #%d: %v", i+1, err)
		}
	}
	for _, dir := range []string{p.DataDir, p.ConfigDir, p.RuntimeDir, p.WorktreesDir, p.LogDir} {
		fi, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
		if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode = %v, want no group/other access", dir, fi.Mode().Perm())
		}
	}
}

func TestSocketAddressOverride(t *testing.T) {
	t.Setenv(EnvSocket, "custom-addr")
	if got := SocketAddress(pathsUnder(t.TempDir())); got != "custom-addr" {
		t.Errorf("SocketAddress = %q, want override", got)
	}
}

func TestSocketAddressDefault(t *testing.T) {
	t.Setenv(EnvSocket, "")
	p := pathsUnder(t.TempDir())
	got := SocketAddress(p)
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(got, `\\.\pipe\multiplex-`) {
			t.Errorf("SocketAddress = %q, want named pipe prefix", got)
		}
		t.Setenv(EnvHome, t.TempDir())
		a := SocketAddress(p)
		t.Setenv(EnvHome, t.TempDir())
		b := SocketAddress(p)
		if a == b {
			t.Errorf("two MPX_HOME values produced the same pipe %q", a)
		}
		return
	}
	if want := filepath.Join(p.RuntimeDir, "mpx.sock"); got != want {
		t.Errorf("SocketAddress = %q, want %q", got, want)
	}
}

func TestSanitizeUser(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"alice", "alice"},
		{"Bob.Smith-2_x", "Bob.Smith-2_x"},
		{"", "user"},
		{"a b/c\\d", "a_b_c_d"},
		{"名前", "__"},
	}
	for _, tt := range tests {
		if got := sanitizeUser(tt.in); got != tt.want {
			t.Errorf("sanitizeUser(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
