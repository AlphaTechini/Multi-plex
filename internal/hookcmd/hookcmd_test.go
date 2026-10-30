package hookcmd

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// fakeDaemon answers hook.event with fn and reports what it saw.
func fakeDaemon(t *testing.T, fn func(protocol.HookParams) agent.HookResponse) (addr string, seen chan protocol.HookParams) {
	t.Helper()
	dir, err := os.MkdirTemp("", "mpxh")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	addr = filepath.Join(dir, "d.sock")
	l, err := net.Listen("unix", addr)
	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	seen = make(chan protocol.HookParams, 8)
	go func() {
		for {
			nc, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer nc.Close()
				enc, dec := protocol.JSON.NewEncoder(nc), protocol.JSON.NewDecoder(nc)
				var f protocol.Frame
				if dec.Decode(&f) != nil {
					return
				}
				var p protocol.HookParams
				_ = json.Unmarshal(f.Data, &p)
				seen <- p
				data, _ := json.Marshal(fn(p))
				_ = enc.Encode(&protocol.Frame{Type: protocol.TypeResponse, ID: f.ID, Data: data})
			}()
		}
	}()
	return addr, seen
}

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const bashPayload = `{"session_id":"claude-1","cwd":"/work","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git push --force"}}`

func run(args []string, stdin string, e map[string]string) (code int, stdout, stderr string) {
	var out, errb strings.Builder
	code = Run(args, strings.NewReader(stdin), &out, &errb, env(e))
	return code, out.String(), errb.String()
}

func TestAllowPassesThrough(t *testing.T) {
	addr, seen := fakeDaemon(t, func(p protocol.HookParams) agent.HookResponse { return agent.HookResponse{Decision: agent.Allow} })
	code, out, errs := run([]string{"pre-tool"}, bashPayload, map[string]string{"MPX_SESSION_ID": "ses_1", "MPX_SOCKET": addr})
	if code != 0 || errs != "" {
		t.Fatalf("code %d stderr %q", code, errs)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Errorf("stdout = %q", out)
	}
	p := <-seen
	if p.SessionID != "ses_1" || p.Adapter != "claude-code" || p.Event.Kind != agent.HookPreTool || p.Event.Command != "git push --force" {
		t.Errorf("daemon saw %+v", p)
	}
}

func TestDenyCarriesReason(t *testing.T) {
	addr, _ := fakeDaemon(t, func(protocol.HookParams) agent.HookResponse {
		return agent.HookResponse{Decision: agent.Deny, Reason: "Denied by the user: no"}
	})
	code, out, _ := run([]string{"pre-tool"}, bashPayload, map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": addr})
	if code != 0 || !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(out, "Denied by the user: no") {
		t.Errorf("code %d stdout %q", code, out)
	}
}

func TestClaudeEventNamesAccepted(t *testing.T) {
	addr, seen := fakeDaemon(t, func(protocol.HookParams) agent.HookResponse { return agent.HookResponse{} })
	code, out, _ := run([]string{"Stop"}, `{"session_id":"c","hook_event_name":"Stop"}`, map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": addr})
	if code != 0 || out != "" {
		t.Errorf("code %d stdout %q", code, out)
	}
	if p := <-seen; p.Event.Kind != agent.HookStop {
		t.Errorf("kind = %s", p.Event.Kind)
	}
}

func TestEveryNeutralEventName(t *testing.T) {
	addr, seen := fakeDaemon(t, func(protocol.HookParams) agent.HookResponse { return agent.HookResponse{} })
	want := map[string]agent.HookKind{
		"pre-tool": agent.HookPreTool, "post-tool": agent.HookPostTool, "permission-request": agent.HookNotification,
		"notification": agent.HookNotification, "stop": agent.HookStop, "prompt-submit": agent.HookPromptSubmit,
		"session-start": agent.HookSessionStart, "session-end": agent.HookSessionEnd,
	}
	for ev, kind := range want {
		run([]string{ev}, `{"session_id":"c","tool_name":"Bash","tool_input":{"command":"ls"}}`, map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": addr})
		if p := <-seen; p.Event.Kind != kind {
			t.Errorf("%s: kind = %s, want %s", ev, p.Event.Kind, kind)
		}
	}
}

func TestOutsideMultiplexIsSilent(t *testing.T) {
	code, out, errs := run([]string{"pre-tool"}, bashPayload, map[string]string{})
	if code != 0 || out != "" || errs != "" {
		t.Errorf("code %d stdout %q stderr %q", code, out, errs)
	}
}

func TestUnreachableDaemonFailsClosedForPreTool(t *testing.T) {
	e := map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": filepath.Join(t.TempDir(), "none.sock")}
	code, out, errs := run([]string{"pre-tool"}, bashPayload, e)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(errs, "Blocked by Multiplex") {
		t.Errorf("stderr must carry the reason for Claude Code: %q", errs)
	}
}

func TestUnreachableDaemonIsSilentForOtherEvents(t *testing.T) {
	e := map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": filepath.Join(t.TempDir(), "none.sock")}
	for _, ev := range []string{"post-tool", "stop", "notification", "prompt-submit", "session-start", "session-end", "permission-request"} {
		code, out, errs := run([]string{ev}, bashPayload, e)
		if code != 0 || out != "" || errs != "" {
			t.Errorf("%s: code %d stdout %q stderr %q", ev, code, out, errs)
		}
	}
}

func TestBadPayloadFailsClosedForPreTool(t *testing.T) {
	addr, _ := fakeDaemon(t, func(protocol.HookParams) agent.HookResponse { return agent.HookResponse{Decision: agent.Allow} })
	e := map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": addr}
	if code, _, _ := run([]string{"pre-tool"}, "{not json", e); code != 2 {
		t.Errorf("garbled pre-tool payload: exit %d, want 2", code)
	}
	if code, out, _ := run([]string{"stop"}, "{not json", e); code != 0 || out != "" {
		t.Errorf("garbled stop payload: exit %d out %q", code, out)
	}
}

func TestUnknownAdapterFailsClosedForPreTool(t *testing.T) {
	e := map[string]string{"MPX_SESSION_ID": "s", "MPX_ADAPTER": "nope"}
	if code, _, _ := run([]string{"pre-tool"}, bashPayload, e); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

// TestPreToolWaitsForSlowApproval: pre-tool has no overall timeout.
func TestPreToolWaitsForSlowApproval(t *testing.T) {
	addr, _ := fakeDaemon(t, func(protocol.HookParams) agent.HookResponse {
		time.Sleep(CallTimeout + 500*time.Millisecond) // longer than any non-gating call may take
		return agent.HookResponse{Decision: agent.Allow}
	})
	if testing.Short() {
		t.Skip("slow")
	}
	code, out, _ := run([]string{"pre-tool"}, bashPayload, map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": addr})
	if code != 0 || !strings.Contains(out, "allow") {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestStdinIsCapped(t *testing.T) {
	addr, seen := fakeDaemon(t, func(protocol.HookParams) agent.HookResponse { return agent.HookResponse{} })
	big := `{"session_id":"c","tool_name":"Bash","tool_input":{"command":"` + strings.Repeat("a", MaxStdin+10) + `"}}`
	run([]string{"post-tool"}, big, map[string]string{"MPX_SESSION_ID": "s", "MPX_SOCKET": addr})
	select {
	case <-seen:
		t.Error("a truncated (invalid) payload reached the daemon")
	case <-time.After(200 * time.Millisecond):
	}
}
