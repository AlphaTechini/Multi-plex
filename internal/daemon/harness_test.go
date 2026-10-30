package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/agent/claudecode"
	_ "github.com/alphatechini/multi-plex/internal/agent/shell"
	"github.com/alphatechini/multi-plex/internal/approval/broker"
	"github.com/alphatechini/multi-plex/internal/approval/typedcode"
	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/daemon/gitremote"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/pty/xptyimpl"
	"github.com/alphatechini/multi-plex/internal/store"
	"github.com/alphatechini/multi-plex/internal/store/memory"
	"github.com/alphatechini/multi-plex/internal/transport"
	"github.com/alphatechini/multi-plex/internal/vcs/gitcli"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

const waitFor = 15 * time.Second

// TestHelperProcess is the "agent" the tests start: the test binary running
// itself. It prints a banner, then echoes each line it reads.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("MPX_TEST_HELPER") != "agent" {
		return
	}
	fmt.Println("helper-agent-ready")
	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			fmt.Printf("\r\nhelper-got:%q\r\n", strings.TrimSpace(string(buf[:n])))
		}
		if err != nil {
			os.Exit(0)
		}
	}
}

// helperAdapter is a hook-capable agent adapter that runs TestHelperProcess.
type helperAdapter struct{ claudecode.Adapter }

func (helperAdapter) Name() string        { return "helper" }
func (helperAdapter) DisplayName() string { return "Helper" }
func (helperAdapter) Prepare(spec agent.LaunchSpec) (agent.Launch, error) {
	env := append(append([]string(nil), spec.Env...), "MPX_TEST_HELPER=agent")
	return agent.Launch{
		Argv: []string{os.Args[0], "-test.run=^TestHelperProcess$"},
		Env:  env,
		Dir:  spec.WorktreePath,
	}, nil
}

func init() { agent.Register(helperAdapter{}) }

// harness is a daemon running in-process on a temp socket with an in-memory
// store, a real git repository, real PTYs and a client connected to it.
type harness struct {
	t      *testing.T
	d      *Daemon
	st     *memory.Store
	broker *broker.Broker
	paths  config.Paths
	addr   string
	repo   string
	origin string
	ws     core.Workspace
	c      *client.Client
	ev     *eventLog
	served chan error
	cancel context.CancelFunc
}

func testAddr(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\mpx-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	}
	dir, err := os.MkdirTemp("", "mpxt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-b", "main")
	gitRun(t, repo, "config", "user.email", "t@t")
	gitRun(t, repo, "config", "user.name", "t")
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	writeFile(t, filepath.Join(repo, "shared.go"), "package main\n\nfunc shared() int { return 1 }\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "init")
	return repo
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newPaths(t *testing.T) config.Paths {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "data")
	return config.Paths{
		DataDir: data, ConfigDir: filepath.Join(root, "config"), RuntimeDir: filepath.Join(root, "run"),
		DBPath: filepath.Join(data, "multiplex.db"), WorktreesDir: filepath.Join(data, "worktrees"), LogDir: filepath.Join(data, "logs"),
	}
}

// newDeps builds daemon dependencies over st.
func newDeps(t *testing.T, st store.Store, paths config.Paths) (Deps, *broker.Broker) {
	t.Helper()
	b := broker.New(nil, typedcode.New(), nil)
	exe, _ := os.Executable()
	return Deps{
		Store: st, VCS: gitcli.New(), Spawner: xptyimpl.New(), Emulator: charmvt.Factory,
		Broker: b, Remotes: gitremote.Git{}, Paths: paths, Settings: config.DefaultSettings(),
		MpxPath: exe, Version: "test",
	}, b
}

// start serves d on a fresh address and connects a client that records events.
func (h *harness) start(d *Daemon) {
	h.t.Helper()
	h.d = d
	h.addr = testAddr(h.t)
	l, err := transport.Listen(h.addr)
	if err != nil {
		h.t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.served = make(chan error, 1)
	go func() { h.served <- d.Serve(ctx, l) }()
	h.c = h.dial()
	h.ev = newEventLog(h.c)
	if err := h.c.Subscribe(context.Background()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) dial() *client.Client {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, h.addr)
	if err != nil {
		h.t.Fatal(err)
	}
	return c
}

// newHarness starts a daemon on a temp repo and opens its workspace.
func newHarness(t *testing.T) *harness { return newHarnessAt(t, nil) }

// newHarnessAt is newHarness with a hook that prepares the repository first.
func newHarnessAt(t *testing.T, prepare func(repo string)) *harness {
	t.Helper()
	h := &harness{t: t, st: memory.New(), paths: newPaths(t), repo: newRepo(t)}
	if prepare != nil {
		prepare(h.repo)
	}
	dp, b := newDeps(t, h.st, h.paths)
	h.broker = b
	h.start(New(dp))
	t.Cleanup(h.stop)
	if err := h.c.Call(context.Background(), protocol.MethodWorkspaceOpen, protocol.WorkspaceOpenParams{Dir: h.repo}, &h.ws); err != nil {
		t.Fatal(err)
	}
	return h
}

// stop shuts the daemon down and waits for it.
func (h *harness) stop() {
	if h.cancel == nil {
		return
	}
	h.c.Close()
	h.cancel()
	select {
	case <-h.served:
	case <-time.After(20 * time.Second):
		h.t.Error("daemon did not shut down")
	}
	h.cancel = nil
}

func (h *harness) call(method string, params, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), waitFor)
	defer cancel()
	return h.c.Call(ctx, method, params, out)
}

func (h *harness) mustCall(method string, params, out any) {
	h.t.Helper()
	if err := h.call(method, params, out); err != nil {
		h.t.Fatalf("%s: %v", method, err)
	}
}

// newSession creates a session with the given agent.
func (h *harness) newSession(agentName, name string) core.Session {
	h.t.Helper()
	var s core.Session
	h.mustCall(protocol.MethodSessionCreate, protocol.SessionCreateParams{
		WorkspaceID: h.ws.ID, Task: "task " + name, Name: name, Agent: agentName, Cols: 100, Rows: 20,
	}, &s)
	return s
}

// hook sends one hook event as the session and returns the response.
func (h *harness) hook(id core.SessionID, ev agent.HookEvent) agent.HookResponse {
	h.t.Helper()
	var resp agent.HookResponse
	h.mustCall(protocol.MethodHook, protocol.HookParams{SessionID: id, Adapter: "helper", Event: ev}, &resp)
	return resp
}

// mcp calls an MCP tool as the session.
func (h *harness) mcp(id core.SessionID, tool string, args any) protocol.MCPResult {
	h.t.Helper()
	b, err := json.Marshal(args)
	if err != nil {
		h.t.Fatal(err)
	}
	var res protocol.MCPResult
	h.mustCall(protocol.MethodMCP, protocol.MCPParams{SessionID: id, Tool: tool, Args: b}, &res)
	return res
}

func (h *harness) detail(id core.SessionID) protocol.SessionDetail {
	h.t.Helper()
	var det protocol.SessionDetail
	h.mustCall(protocol.MethodSessionGet, protocol.SessionRef{SessionID: id}, &det)
	return det
}

// waitState polls until the session reaches state.
func (h *harness) waitState(id core.SessionID, state core.SessionState) core.Session {
	h.t.Helper()
	deadline := time.Now().Add(waitFor)
	var s core.Session
	for time.Now().Before(deadline) {
		s = h.detail(id).Session
		if s.State == state {
			return s
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("session %s: state %s (%s), want %s", id, s.State, s.StateDetail, state)
	return s
}

// eventLog records every event a client receives.
type eventLog struct {
	mu     sync.Mutex
	cond   *sync.Cond
	events []client.Event
}

func newEventLog(c *client.Client) *eventLog {
	l := &eventLog{}
	l.cond = sync.NewCond(&l.mu)
	go func() {
		for ev := range c.Events() {
			l.mu.Lock()
			l.events = append(l.events, ev)
			l.mu.Unlock()
			l.cond.Broadcast()
		}
		l.cond.Broadcast()
	}()
	return l
}

// wait returns the first event of topic, from the start of the log, for which
// match returns true.
func (l *eventLog) wait(t *testing.T, topic string, match func(json.RawMessage) bool) json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(waitFor)
	timer := time.AfterFunc(waitFor, l.cond.Broadcast)
	defer timer.Stop()
	l.mu.Lock()
	defer l.mu.Unlock()
	next := 0
	for {
		for ; next < len(l.events); next++ {
			if ev := l.events[next]; ev.Topic == topic && (match == nil || match(ev.Data)) {
				return ev.Data
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s event within %s (got %d events)", topic, waitFor, len(l.events))
		}
		l.cond.Wait()
	}
}

// screen builds a pane's text from its frames in the log so far.
func (l *eventLog) screenText(pane core.PaneID) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows := map[int]string{}
	for _, ev := range l.events {
		if ev.Topic != protocol.TopicPaneFrame {
			continue
		}
		var f protocol.PaneFrame
		if json.Unmarshal(ev.Data, &f) != nil || f.PaneID != pane {
			continue
		}
		if f.Full {
			rows = map[int]string{}
		}
		for _, r := range f.Changed {
			rows[r.Y] = ansi.Strip(r.Text)
		}
	}
	var b strings.Builder
	for y := 0; y < 200; y++ {
		if s, ok := rows[y]; ok {
			b.WriteString(strings.TrimRight(s, " "))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// waitScreen waits until the pane's reconstructed screen contains want.
func (l *eventLog) waitScreen(t *testing.T, pane core.PaneID, want string) string {
	t.Helper()
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {
		if s := l.screenText(pane); strings.Contains(s, want) {
			return s
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pane %s never showed %q; screen:\n%s", pane, want, l.screenText(pane))
	return ""
}

func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}
