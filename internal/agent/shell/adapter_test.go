package shell

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
)

func TestRegistered(t *testing.T) {
	a, ok := agent.Get("shell")
	if !ok {
		t.Fatal("shell adapter not registered")
	}
	if a.Name() != "shell" || a.DisplayName() == "" {
		t.Errorf("Name/DisplayName = %q/%q", a.Name(), a.DisplayName())
	}
	if a.Capabilities() != (agent.Capabilities{}) {
		t.Errorf("Capabilities = %+v, want none", a.Capabilities())
	}
}

func TestShellArgv(t *testing.T) {
	has := func(names ...string) func(string) (string, error) {
		return func(exe string) (string, error) {
			if slices.Contains(names, exe) {
				return `C:\bin\` + exe, nil
			}
			return "", errors.New("not found")
		}
	}
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	tests := []struct {
		name    string
		goos    string
		env     map[string]string
		look    func(string) (string, error)
		want    []string
		wantErr bool
	}{
		{"windows pwsh", "windows", nil, has("pwsh.exe", "powershell.exe"), []string{`C:\bin\pwsh.exe`, "-NoLogo"}, false},
		{"windows fallback", "windows", nil, has("powershell.exe"), []string{`C:\bin\powershell.exe`, "-NoLogo"}, false},
		{"windows never cmd", "windows", map[string]string{"SHELL": "cmd.exe", "ComSpec": "cmd.exe"}, has("cmd.exe"), nil, true},
		{"linux SHELL", "linux", map[string]string{"SHELL": "/usr/bin/zsh"}, has(), []string{"/usr/bin/zsh"}, false},
		{"darwin default", "darwin", nil, has(), []string{"/bin/sh"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shellArgv(tt.goos, env(tt.env), tt.look)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("argv = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrepare(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	spec := agent.LaunchSpec{
		Session:      core.Session{ID: "ses_abc"},
		WorktreePath: "/work/tree",
		Env:          []string{"PATH=/usr/bin", "MPX_SESSION_ID=stale", "A=1"},
	}
	l, err := Adapter{}.Prepare(spec)
	if err != nil {
		t.Skipf("no usable shell on this machine: %v", err)
	}
	if l.Dir != "/work/tree" || len(l.Argv) == 0 {
		t.Errorf("Launch = %+v", l)
	}
	n := 0
	for _, kv := range l.Env {
		if strings.HasPrefix(kv, "MPX_SESSION_ID=") {
			n++
			if kv != "MPX_SESSION_ID=ses_abc" {
				t.Errorf("MPX_SESSION_ID = %q", kv)
			}
		}
	}
	if n != 1 {
		t.Errorf("MPX_SESSION_ID appears %d times in %v", n, l.Env)
	}
	if !slices.Contains(l.Env, "A=1") || !slices.Contains(l.Env, "PATH=/usr/bin") {
		t.Errorf("base env lost: %v", l.Env)
	}
}

func TestInput(t *testing.T) {
	a := Adapter{}
	if got := string(a.TypeMessage("ls -la")); got != "ls -la\r" {
		t.Errorf("TypeMessage = %q", got)
	}
	if got := string(a.Interrupt()); got != "\x03" {
		t.Errorf("Interrupt = %q", got)
	}
	if b, ok := a.SideQuestion("q"); ok || b != nil {
		t.Errorf("SideQuestion = %q, %v", b, ok)
	}
	if _, err := a.ParseHook("pre-tool", nil); err == nil {
		t.Error("ParseHook should fail")
	}
	if out, code := a.EncodeHookResponse(agent.HookEvent{}, agent.HookResponse{}); out != nil || code != 0 {
		t.Errorf("EncodeHookResponse = %q, %d", out, code)
	}
	if _, code := a.FailClosed("x"); code == 0 {
		t.Error("FailClosed must not exit 0")
	}
}
