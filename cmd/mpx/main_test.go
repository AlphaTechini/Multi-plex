package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/daemon"
	"github.com/alphatechini/multi-plex/internal/hookcmd"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// envAsMPX makes the test binary behave as the mpx binary, so tests can start
// daemons, hooks and MCP servers as real child processes.
const envAsMPX = "MPX_TEST_AS_MPX"

func TestMain(m *testing.M) {
	if os.Getenv(envAsMPX) == "1" {
		if len(os.Args) > 1 && os.Args[1] == "hook" {
			os.Exit(hookcmd.Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
		}
		os.Exit(run(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// mpxEnv gives the process an isolated MPX_HOME and the as-mpx switch.
func mpxEnv(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("", "mpxm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv(config.EnvHome, home)
	t.Setenv(config.EnvSocket, "")
	t.Setenv(envAsMPX, "1")
	return home
}

func mpx(t *testing.T, stdin string, args ...string) (stdout string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return out.String(), ee.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), 0
}

func TestVersionAndUnknownCommand(t *testing.T) {
	mpxEnv(t)
	out, code := mpx(t, "", "version")
	if code != 0 || !strings.HasPrefix(out, "mpx ") {
		t.Errorf("version: code %d out %q", code, out)
	}
	if _, code := mpx(t, "", "frobnicate"); code != 2 {
		t.Errorf("unknown command: exit %d, want 2", code)
	}
}

func TestDoctor(t *testing.T) {
	mpxEnv(t)
	out, code := mpx(t, "", "doctor")
	if code != 0 {
		t.Fatalf("doctor exit %d:\n%s", code, out)
	}
	for _, want := range []string{"data", "socket", "daemon", "not running", "git", "approval", "terminal", "agent shell"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output lacks %q:\n%s", want, out)
		}
	}
}

func TestHookOutsideMultiplexIsSilent(t *testing.T) {
	mpxEnv(t)
	t.Setenv("MPX_SESSION_ID", "")
	out, code := mpx(t, `{"tool_name":"Bash"}`, "hook", "pre-tool")
	if code != 0 || out != "" {
		t.Errorf("code %d out %q", code, out)
	}
}

// TestRealProcesses starts a real daemon with EnsureDaemon, then talks to it
// through a real `mpx hook` process and a real `mpx mcp` process.
func TestRealProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("starts real processes")
	}
	home := mpxEnv(t)
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	addr, err := daemon.EnsureDaemon(ctx)
	if err != nil {
		t.Fatalf("EnsureDaemon: %v", err)
	}
	if want := filepath.Join(home, "run", "mpx.sock"); addr != want && !strings.Contains(addr, "pipe") {
		t.Errorf("addr = %q, want %q", addr, want)
	}
	c, err := client.Dial(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	t.Cleanup(func() {
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		_ = c.Shutdown(sctx)
		select {
		case <-c.Done():
		case <-time.After(10 * time.Second):
			t.Error("daemon did not shut down")
		}
	})

	hello, err := c.Hello(ctx, "test")
	if err != nil || hello.PID == 0 || hello.PID == os.Getpid() {
		t.Fatalf("hello = %+v, %v", hello, err)
	}
	// A second EnsureDaemon finds the same daemon.
	if again, err := daemon.EnsureDaemon(ctx); err != nil || again != addr {
		t.Fatalf("second EnsureDaemon = %q, %v", again, err)
	}

	if err := c.Subscribe(ctx, protocol.TopicNote); err != nil {
		t.Fatal(err)
	}
	ws, err := c.OpenWorkspace(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := c.CreateSession(ctx, protocol.SessionCreateParams{WorkspaceID: ws.ID, Task: "real processes", Agent: "shell"})
	if err != nil {
		t.Fatal(err)
	}
	// The shell inside the session got the variables the hook and MCP processes need.
	if sess.State == core.StateError {
		t.Fatalf("session failed to start: %s", sess.StateDetail)
	}

	// `mpx hook pre-tool` as the agent CLI would run it.
	t.Setenv("MPX_SESSION_ID", string(sess.ID))
	t.Setenv("MPX_SOCKET", addr)
	payload := `{"session_id":"c1","cwd":"` + filepath.ToSlash(sess.WorktreePath) + `","hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"` + filepath.ToSlash(filepath.Join(sess.WorktreePath, "README.md")) + `"}}`
	out, code := mpx(t, payload, "hook", "pre-tool")
	if code != 0 || !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Errorf("mpx hook: code %d out %q", code, out)
	}

	// `mpx mcp` over stdio.
	srv := exec.Command(os.Args[0], "mcp")
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "0"}, nil).Connect(ctx, &mcp.CommandTransport{Command: srv}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "note_to_user", Arguments: map[string]any{"title": "from a real mpx mcp"}})
	if err != nil || res.IsError {
		t.Fatalf("note_to_user: %v %+v", err, res)
	}
	select {
	case ev := <-c.Events():
		if ev.Topic != protocol.TopicNote || !strings.Contains(string(ev.Data), "from a real mpx mcp") {
			t.Errorf("event = %s %s", ev.Topic, ev.Data)
		}
	case <-time.After(10 * time.Second):
		t.Error("no note.added event")
	}
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 10 {
		t.Errorf("tools = %d, %v", len(tools.Tools), err)
	}
}
