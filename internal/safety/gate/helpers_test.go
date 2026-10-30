package gate

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

// Fixed, host-independent worktrees so the same tables run on every OS.
const (
	linuxRoot = "/home/u/proj"
	winRoot   = `C:\work\proj`
)

func linuxGate() *Gate {
	d := pathpolicy.Dirs{Home: "/home/u", Temp: "/tmp", Cache: "/home/u/.cache", Config: "/home/u/.config"}
	return &Gate{Policy: pathpolicy.DefaultPolicyFor("linux", linuxRoot, d), Shell: "bash"}
}

func winGate() *Gate {
	d := pathpolicy.Dirs{Home: `C:\Users\u`, Temp: `C:\Users\u\AppData\Local\Temp`, Cache: `C:\Users\u\AppData\Local`,
		Config: `C:\Users\u\AppData\Roaming`, AppData: `C:\Users\u\AppData\Roaming`, LocalAppData: `C:\Users\u\AppData\Local`}
	return &Gate{Policy: pathpolicy.DefaultPolicyFor("windows", winRoot, d), Shell: "powershell"}
}

func sh(root, cmd string) agent.HookEvent {
	return agent.HookEvent{Kind: agent.HookPreTool, Cwd: root, ToolName: "Bash", ToolClass: agent.ToolShell, Command: cmd}
}

func write(root string, paths ...string) agent.HookEvent {
	return agent.HookEvent{Kind: agent.HookPreTool, Cwd: root, ToolName: "Write", ToolClass: agent.ToolWrite, Paths: paths}
}

func read(root string, paths ...string) agent.HookEvent {
	return agent.HookEvent{Kind: agent.HookPreTool, Cwd: root, ToolName: "Read", ToolClass: agent.ToolRead, Paths: paths}
}

type gateCase struct {
	name  string
	ev    agent.HookEvent
	layer string // expected layer when gated
	rule  string // expected rule id prefix when gated ("" = any)
}

func expectGated(t *testing.T, g *Gate, cases []gateCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := g.Evaluate(c.ev)
			if v.Decision != DecisionGate {
				t.Fatalf("not gated: %+v\nevent: %+v", v, c.ev)
			}
			if c.layer != "" && v.Layer != c.layer {
				t.Errorf("layer = %q, want %q (verdict %+v)", v.Layer, c.layer, v)
			}
			if c.rule != "" && !hasPrefix(v.Rule, c.rule) {
				t.Errorf("rule = %q, want prefix %q", v.Rule, c.rule)
			}
			if v.Reason == "" || v.Rule == "" || v.Kind == "" {
				t.Errorf("incomplete verdict: %+v", v)
			}
		})
	}
}

func expectAllowed(t *testing.T, g *Gate, cases []gateCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if v := g.Evaluate(c.ev); v.Decision != DecisionAllow {
				t.Fatalf("unexpected prompt: %+v\nevent: %+v", v, c.ev)
			}
		})
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
