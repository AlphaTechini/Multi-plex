package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hookOpts are the flags of `mpx bench hook`.
type hookOpts struct {
	n         int
	procs     int
	tool      string
	useDaemon bool
}

func hookFlags(fs *flag.FlagSet) *hookOpts {
	o := &hookOpts{}
	fs.IntVar(&o.n, "n", 500, "round trips over one connection")
	fs.IntVar(&o.procs, "procs", 100, "separate 'mpx hook' processes to time (0 skips)")
	fs.StringVar(&o.tool, "tool", "read", "tool to send: read (hot path) or shell (includes the checkpoint)")
	fs.BoolVar(&o.useDaemon, "use-daemon", false, "measure the daemon already running at the default address instead of one started in-process")
	return o
}

// runHook measures the pre-tool hook path (docs/03-performance.md: under
// 10 ms on Linux and 25 ms on Windows for an allowed call).
func runHook(o *hookOpts) (*Result, error) {
	if o.tool != "read" && o.tool != "shell" {
		return nil, fmt.Errorf("--tool must be read or shell, got %q", o.tool)
	}
	env, err := newHookEnv(o.useDaemon)
	if err != nil {
		return nil, err
	}
	defer env.close()

	res := newResult("hook")
	budget, limit := "< 10ms p50", BudgetHookLinux
	if runtime.GOOS == "windows" {
		budget, limit = "< 25ms p50", BudgetHookWindows
	}
	res.note("daemon: %s; tool: %s; session worktree: %s", env.kind, o.tool, env.worktree)

	ev := env.event(o.tool)
	ctx := context.Background()
	for i := 0; i < 20; i++ { // warm up the connection and the daemon's caches
		if _, err := env.c.Hook(ctx, protocol.HookParams{SessionID: env.session, Adapter: "claude-code", Event: ev}); err != nil {
			return nil, fmt.Errorf("warm-up hook call: %w", err)
		}
	}
	samples := make([]time.Duration, 0, o.n)
	for i := 0; i < o.n; i++ {
		t0 := time.Now()
		resp, err := env.c.Hook(ctx, protocol.HookParams{SessionID: env.session, Adapter: "claude-code", Event: ev})
		d := time.Since(t0)
		if err != nil {
			return nil, fmt.Errorf("hook call %d: %w", i, err)
		}
		if resp.Decision != agent.Allow {
			return nil, fmt.Errorf("hook call %d was not allowed: %+v", i, resp)
		}
		samples = append(samples, d)
	}
	res.addTiming("round trip, one connection (daemon handling + socket)", samples, budget, limit, func(s Stats) float64 { return s.P50 })

	if o.procs > 0 {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		payload := env.payload(o.tool)
		ps := make([]time.Duration, 0, o.procs)
		for i := 0; i < o.procs+5; i++ {
			d, err := runHookProcess(exe, env, payload)
			if err != nil {
				return nil, fmt.Errorf("mpx hook process %d: %w", i, err)
			}
			if i >= 5 { // the first few warm the page cache
				ps = append(ps, d)
			}
		}
		res.addTiming("full 'mpx hook pre-tool' process (spawn + connect + round trip + exit)", ps, budget, limit, func(s Stats) float64 { return s.P50 })
	}
	return res, nil
}

// runHookProcess runs `exe hook pre-tool` once, as Claude Code would, and
// returns how long it took.
func runHookProcess(exe string, env *hookEnv, payload []byte) (time.Duration, error) {
	cmd := exec.Command(exe, "hook", "pre-tool")
	cmd.Stdin = bytes.NewReader(payload)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.Env = append(cleanEnv(os.Environ()), "MPX_SESSION_ID="+string(env.session), "MPX_SOCKET="+env.addr, "MPX_ADAPTER=claude-code")
	t0 := time.Now()
	err := cmd.Run()
	d := time.Since(t0)
	if err != nil {
		return 0, fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(errb.String()))
	}
	if !strings.Contains(out.String(), `"permissionDecision":"allow"`) {
		return 0, fmt.Errorf("not allowed: %s", strings.TrimSpace(out.String()))
	}
	return d, nil
}

// cleanEnv removes the Multiplex variables of the environment the benchmark
// itself runs in.
func cleanEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if k == "MPX_SESSION_ID" || k == "MPX_SOCKET" || k == "MPX_ADAPTER" {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// hookEnv is a daemon, a connection and one session to send hooks as.
type hookEnv struct {
	kind     string
	addr     string
	c        *client.Client
	session  core.SessionID
	worktree string

	cleanup []func()
}

func (e *hookEnv) close() {
	for i := len(e.cleanup) - 1; i >= 0; i-- {
		e.cleanup[i]()
	}
}

// event builds the pre-tool event for a tool.
func (e *hookEnv) event(tool string) agent.HookEvent {
	if tool == "shell" {
		return agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell, Command: "go test ./...", Cwd: e.worktree}
	}
	return agent.HookEvent{
		Kind: agent.HookPreTool, ToolName: "Read", ToolClass: agent.ToolRead,
		Paths: []string{filepath.Join(e.worktree, "README.md")}, Cwd: e.worktree,
	}
}

// payload is the stdin JSON Claude Code would send for the event.
func (e *hookEnv) payload(tool string) []byte {
	in := map[string]any{"file_path": filepath.Join(e.worktree, "README.md")}
	name := "Read"
	if tool == "shell" {
		name, in = "Bash", map[string]any{"command": "go test ./..."}
	}
	b, _ := json.Marshal(map[string]any{
		"session_id": "bench", "cwd": e.worktree, "hook_event_name": "PreToolUse", "tool_name": name, "tool_input": in,
	})
	return b
}

// newHookEnv starts (or finds) a daemon and creates a bench session in a
// throw-away git repository.
func newHookEnv(useDaemon bool) (*hookEnv, error) {
	e := &hookEnv{}
	ok := false
	defer func() {
		if !ok {
			e.close()
		}
	}()

	repo, cleanup, err := makeRepo()
	if err != nil {
		return nil, err
	}
	e.cleanup = append(e.cleanup, cleanup)

	if useDaemon {
		paths, err := config.DefaultPaths()
		if err != nil {
			return nil, err
		}
		e.addr, e.kind = config.SocketAddress(paths), "running daemon at "+config.SocketAddress(paths)
	} else {
		addr, stop, err := startInProcessDaemon()
		if err != nil {
			return nil, err
		}
		e.addr, e.kind = addr, "in-process daemon on a temp socket"
		e.cleanup = append(e.cleanup, stop)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, e.addr)
	if err != nil {
		if useDaemon {
			return nil, fmt.Errorf("no daemon at %s (start one with 'mpx daemon'): %w", e.addr, err)
		}
		return nil, err
	}
	e.c = c
	e.cleanup = append(e.cleanup, func() { _ = c.Close() })

	ws, err := c.OpenWorkspace(ctx, repo)
	if err != nil {
		return nil, err
	}
	sess, err := c.CreateSession(ctx, protocol.SessionCreateParams{WorkspaceID: ws.ID, Task: "bench hook", Name: "bench-hook", Agent: "shell", Cols: 80, Rows: 24})
	if err != nil {
		return nil, err
	}
	e.session, e.worktree = sess.ID, sess.WorktreePath
	e.cleanup = append(e.cleanup, func() {
		rctx, rcancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer rcancel()
		if err := c.RemoveSession(rctx, sess.ID); err != nil && !errors.Is(err, client.ErrClosed) {
			fmt.Fprintf(os.Stderr, "mpx bench: remove bench session: %v\n", err)
		}
	})
	ok = true
	return e, nil
}

// makeRepo creates a git repository with one commit in a temp directory.
func makeRepo() (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "mpxbench")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("bench\n"), 0o644); err != nil {
		cleanup()
		return "", nil, err
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.name=bench", "-c", "user.email=bench@localhost", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(string(out)))
		}
	}
	return repo, cleanup, nil
}
