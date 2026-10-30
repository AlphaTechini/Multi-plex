package daemon

import (
	"path/filepath"
	"runtime"
	"slices"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/safety/classify"
	"github.com/alphatechini/multi-plex/internal/safety/gate"
	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

// policyFor returns the Layer 2 and 3 path policy of a session's worktree:
// the defaults plus the user's extra deny-read entries. Defaults are never
// removed (docs/04-safety.md).
func (d *Daemon) policyFor(info core.Session) pathpolicy.Policy {
	p := pathpolicy.DefaultPolicy(info.WorktreePath)
	p.DenyRead = append(slices.Clone(p.DenyRead), d.settings.ExtraDenyRead...)
	return p
}

// buildGate assembles the gate of a session: the default classifier rules
// plus the user's and the workspace's safety-rules.json, and the path policy.
// A bad rules file is logged and ignored, keeping the safe defaults.
func (d *Daemon) buildGate(info core.Session, ws core.Workspace) *gate.Gate {
	rules, err := classify.Load(
		filepath.Join(d.paths.ConfigDir, "safety-rules.json"),
		filepath.Join(ws.Root, ".multiplex", "safety-rules.json"),
	)
	if err != nil {
		d.log.Warn("safety rules file ignored", "session", info.ID, "err", err)
		d.toast("warning", "A safety-rules.json file is invalid and was ignored; default rules stay active.")
	}
	g := &gate.Gate{
		Rules:         rules,
		Policy:        d.policyFor(info),
		CurrentBranch: info.Branch,
	}
	if runtime.GOOS == "windows" {
		g.Shell = "powershell"
	} else {
		g.Shell = "bash"
	}
	return g
}

// gateFor returns the session's gate, building it on first use.
func (d *Daemon) gateFor(s *session) *gate.Gate {
	s.mu.Lock()
	g := s.gate
	info := s.info
	s.mu.Unlock()
	if g != nil {
		return g
	}
	ws, err := d.workspace(d.ctx, info.WorkspaceID)
	if err != nil {
		ws = core.Workspace{}
	}
	g = d.buildGate(info, ws)
	s.mu.Lock()
	if s.gate == nil {
		s.gate = g
	} else {
		g = s.gate
	}
	s.mu.Unlock()
	return g
}
