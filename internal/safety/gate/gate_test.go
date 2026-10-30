package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/safety/classify"
	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

func TestVerdictDetails(t *testing.T) {
	g := linuxGate()

	v := g.Evaluate(sh(linuxRoot+"/sub", "git push --force origin x"))
	if v.Decision != DecisionGate || v.Kind != core.ApprovalPush || v.Layer != LayerCommand || v.Rule != "git-push-force" || v.Cwd != "sub" {
		t.Errorf("push verdict = %+v", v)
	}

	v = g.Evaluate(sh(linuxRoot, "terraform destroy"))
	if v.Kind != core.ApprovalCommand || v.Cwd != "." {
		t.Errorf("terraform verdict = %+v", v)
	}

	v = g.Evaluate(write(linuxRoot, "/etc/hosts", "/home/u/proj/ok.go", "/home/u/.bashrc"))
	if v.Kind != core.ApprovalFileWrite || v.Layer != LayerWrites {
		t.Errorf("write verdict = %+v", v)
	}
	if want := []string{"/etc/hosts", "/home/u/.bashrc"}; !reflect.DeepEqual(v.Paths, want) {
		t.Errorf("Paths = %v, want %v", v.Paths, want)
	}

	v = g.Evaluate(write(linuxRoot, "/home/u/proj/.claude/settings.json"))
	if !reflect.DeepEqual(v.Paths, []string{".claude/settings.json"}) || v.Rule != pathpolicy.RuleProtectedConfig {
		t.Errorf("settings verdict = %+v", v)
	}

	v = g.Evaluate(read(linuxRoot, "~/.aws/credentials"))
	if v.Kind != core.ApprovalFileRead || v.Layer != LayerCredentials || !reflect.DeepEqual(v.Paths, []string{"/home/u/.aws/credentials"}) {
		t.Errorf("read verdict = %+v", v)
	}

	v = g.Evaluate(sh(linuxRoot, "cat ~/.ssh/id_rsa"))
	if v.Kind != core.ApprovalCommand || v.Layer != LayerCredentials || v.Rule != pathpolicy.RuleDenyRead || len(v.Paths) != 1 {
		t.Errorf("shell credential verdict = %+v", v)
	}
	if v.Allowed() {
		t.Error("gated verdict reports Allowed")
	}
}

func TestProtectedBranchesAndShell(t *testing.T) {
	g := linuxGate()
	g.ProtectedBranches = []string{"release"}
	if v := g.Evaluate(sh(linuxRoot, "git push origin release")); v.Decision != DecisionGate {
		t.Errorf("custom protected branch not gated: %+v", v)
	}
	if v := g.Evaluate(sh(linuxRoot, "git push origin main")); v.Rule != "git-push-any" {
		t.Errorf("main should not be a protected branch with a custom list (only the catch-all push rule applies): %+v", v)
	}
	g.CurrentBranch = "release"
	if v := g.Evaluate(sh(linuxRoot, "git push")); v.Decision != DecisionGate {
		t.Errorf("bare push on protected current branch: %+v", v)
	}
}

func TestCustomRules(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rules.json")
	body := `{"version":1,"rules":[{"id":"deploy-prod","description":"Production deploys need a human","class":"dangerous","match":{"argv":["./scripts/deploy","**","--prod"]}}]}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	rules, err := classify.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	g := linuxGate()
	g.Rules = rules
	v := g.Evaluate(sh(linuxRoot, "./scripts/deploy --prod"))
	if v.Decision != DecisionGate || v.Rule != "deploy-prod" {
		t.Errorf("custom rule verdict = %+v", v)
	}
	if v := g.Evaluate(sh(linuxRoot, "./scripts/deploy --staging")); v.Decision != DecisionAllow {
		t.Errorf("staging gated: %+v", v)
	}
	if v := g.Evaluate(sh(linuxRoot, "git push -f")); v.Decision != DecisionGate {
		t.Errorf("defaults lost when custom rules loaded: %+v", v)
	}
}

func TestProtectedWorkspacePaths(t *testing.T) {
	g := linuxGate()
	g.Policy.Protected = []string{".env", "data/*.db"}
	expectGated(t, g, []gateCase{
		{"write env", write(linuxRoot, ".env"), LayerWrites, "protected-path"},
		{"write db", write(linuxRoot, "data/app.db"), LayerWrites, "protected-path"},
		{"shell redirect to env", sh(linuxRoot, "echo KEY=1 > .env"), LayerWrites, "protected-path"},
		{"shell rm env", sh(linuxRoot, "rm .env"), LayerWrites, "protected-path"},
		{"shell tee db", sh(linuxRoot, "echo x | tee data/app.db"), LayerWrites, "protected-path"},
	})
	expectAllowed(t, g, []gateCase{
		{"write other", write(linuxRoot, "data/schema.sql"), "", ""},
		{"rm other", sh(linuxRoot, "rm data/old.sql"), "", ""},
	})
}

func TestScriptsInTheWorktreeAreScanned(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &Gate{Policy: pathpolicy.Policy{WorktreeRoot: wt, Home: filepath.Join(root, "home")}}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(wt, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("clean.sh", "#!/bin/sh\nset -e\nrm -rf "+filepath.Join(root, "home")+"/Documents\n")
	write("push.sh", "git push --force origin main\n")
	write("ok.sh", "#!/bin/sh\ngo build ./...\nrm -rf build\n")
	write("big.sh", string(make([]byte, maxScriptSize+10)))

	for _, c := range []struct {
		cmd   string
		gated bool
		rule  string
	}{
		{"sh clean.sh", true, "delete-outside-worktree"},
		{"bash ./clean.sh", true, "delete-outside-worktree"},
		{"./clean.sh", true, "delete-outside-worktree"},
		{"source clean.sh", true, "delete-outside-worktree"},
		{"sh push.sh", true, "git-push-force"},
		{"sh ok.sh", false, ""},
		{"sh missing.sh", false, ""},
		{"sh big.sh", false, ""},
	} {
		ev := agent.HookEvent{Kind: agent.HookPreTool, Cwd: wt, ToolClass: agent.ToolShell, Command: c.cmd}
		if runtime.GOOS == "windows" && (c.cmd == "./clean.sh") {
			continue
		}
		v := g.Evaluate(ev)
		if (v.Decision == DecisionGate) != c.gated || (c.gated && v.Rule != c.rule) {
			t.Errorf("%q: verdict %+v, want gated=%v rule=%q", c.cmd, v, c.gated, c.rule)
		}
		if c.gated && (v.Layer != LayerCommand || len(v.Paths) != 1) {
			t.Errorf("%q: script verdict missing details: %+v", c.cmd, v)
		}
	}
}

func TestSymlinkEscapeThroughFileTool(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	wt := filepath.Join(base, "wt")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{wt, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(wt, "link")); err != nil {
		t.Skip(err)
	}
	g := &Gate{Policy: pathpolicy.DefaultPolicy(wt)}
	g.Policy.WritableExtra = nil
	if v := g.Evaluate(write(wt, "link/file.txt")); v.Decision != DecisionGate || v.Rule != pathpolicy.RuleOutsideWorktree {
		t.Errorf("write through symlink = %+v", v)
	}
	if v := g.Evaluate(write(wt, "real.txt")); v.Decision != DecisionAllow {
		t.Errorf("plain write = %+v", v)
	}
}

func TestDescribe(t *testing.T) {
	g := linuxGate()
	ev := sh(linuxRoot+"/svc", "kubectl delete pod api-1 -n prod")
	ev.ToolName = "Bash"
	req := Describe(g.Evaluate(ev), ev)
	want := core.ApprovalRequest{Kind: core.ApprovalCommand, Tool: "Bash", Command: "kubectl delete pod api-1 -n prod",
		Cwd: "svc", Rule: "kubectl-change", Layer: LayerCommand}
	if !reflect.DeepEqual(req, want) {
		t.Errorf("Describe = %+v\nwant     %+v", req, want)
	}

	ev = write(linuxRoot, "/etc/hosts")
	req = Describe(g.Evaluate(ev), ev)
	if req.Kind != core.ApprovalFileWrite || !reflect.DeepEqual(req.Paths, []string{"/etc/hosts"}) || req.Tool != "Write" || req.Layer != LayerWrites {
		t.Errorf("file Describe = %+v", req)
	}

	// An empty verdict still produces a sensible kind.
	req = Describe(Verdict{}, read(linuxRoot, "x"))
	if req.Kind != core.ApprovalFileRead {
		t.Errorf("kind fallback = %v", req.Kind)
	}
	if !req.At.IsZero() || req.ID != "" || req.SessionID != "" {
		t.Error("Describe must leave ID, SessionID and At to the caller")
	}
}

func TestRateCounter(t *testing.T) {
	var c RateCounter
	t0 := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	c.Add("a", t0)
	c.Add("a", t0.Add(20*time.Minute))
	c.Add("a", t0.Add(50*time.Minute))
	c.Add("b", t0.Add(10*time.Minute))
	for _, tc := range []struct {
		session core.SessionID
		at      time.Duration
		want    int
	}{
		{"a", 55 * time.Minute, 3},
		{"a", 59*time.Minute + 59*time.Second, 3},
		{"a", 60*time.Minute + time.Second, 2}, // t0 slid out
		{"a", 85 * time.Minute, 1},
		{"a", 111 * time.Minute, 0},
		{"b", 55 * time.Minute, 1},
		{"nobody", 55 * time.Minute, 0},
	} {
		if got := c.PerHour(tc.session, t0.Add(tc.at)); got != tc.want {
			t.Errorf("PerHour(%s, +%v) = %d, want %d", tc.session, tc.at, got, tc.want)
		}
	}
	c = RateCounter{}
	c.Add("a", t0)
	c.Add("b", t0.Add(time.Minute))
	if got := c.TotalPerHour(t0.Add(2 * time.Minute)); got != 2 {
		t.Errorf("TotalPerHour = %d", got)
	}
	c.Forget("a")
	if got := c.TotalPerHour(t0.Add(2 * time.Minute)); got != 1 {
		t.Errorf("TotalPerHour after Forget = %d", got)
	}
	// A query in the past does not count future events.
	if got := c.PerHour("b", t0); got != 0 {
		t.Errorf("future event counted: %d", got)
	}
}

func TestRateCounterConcurrent(t *testing.T) {
	var c RateCounter
	now := time.Now()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			for j := 0; j < 200; j++ {
				c.Add("s", now)
				c.PerHour("s", now)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if got := c.PerHour("s", now); got != 1600 {
		t.Errorf("count = %d", got)
	}
}

func TestEvaluateIgnoresNonPreTool(t *testing.T) {
	g := linuxGate()
	for _, k := range []agent.HookKind{agent.HookPostTool, agent.HookStop, agent.HookNotification, agent.HookPromptSubmit} {
		ev := sh(linuxRoot, "rm -rf ~")
		ev.Kind = k
		if v := g.Evaluate(ev); v.Decision != DecisionAllow {
			t.Errorf("%s event gated: %+v", k, v)
		}
	}
	// A shell event without a tool class but with a command is still checked.
	ev := agent.HookEvent{Kind: agent.HookPreTool, Command: "rm -rf ~", Cwd: linuxRoot}
	if v := g.Evaluate(ev); v.Decision != DecisionGate {
		t.Errorf("classless command not gated: %+v", v)
	}
}

func BenchmarkEvaluate(b *testing.B) {
	g := linuxGate()
	evs := []agent.HookEvent{
		sh(linuxRoot, "go test ./... && git status"),
		sh(linuxRoot, `bash -c "cd internal && go build ./..."`),
		write(linuxRoot, "internal/x/y.go"),
		read(linuxRoot, "/usr/include/stdio.h"),
		sh(linuxRoot, "pnpm run build 2>&1 | tail -20"),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g.Evaluate(evs[i%len(evs)])
	}
}
