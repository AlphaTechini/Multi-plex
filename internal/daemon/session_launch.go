package daemon

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/role"
)

// launchAgent starts the session's agent in its worktree. A failure never
// returns an error: the session goes to the error state with a StateDetail
// that says what to do (for example how to install a missing agent).
// first is true for the session's first launch, which delivers the task: on
// the command line for adapters with InitialPrompt, typed after the agent
// starts for the others.
func (d *Daemon) launchAgent(ctx context.Context, s *session, ws core.Workspace, cols, rows int, first bool) {
	info := s.snap()
	a, err := adapterFor(info)
	if err != nil {
		d.launchFailed(s, nil, err)
		return
	}

	var r role.Role
	roles, rerr := role.Load(ws.Root, filepath.Join(d.paths.ConfigDir, "roles"))
	if rerr != nil {
		d.log.Warn("role files ignored", "err", rerr)
	}
	if found, ok := roles[info.Role]; ok {
		r = found
	}

	portBase := d.ports.Alloc(info.ID)
	spec := agent.LaunchSpec{
		Session:       info,
		WorkspaceRoot: ws.Root,
		WorktreePath:  info.WorktreePath,
		ConfigDir:     d.sessionConfigDir(info.ID),
		MpxPath:       d.mpxPath,
		HookCommand:   d.hookCmd,
		Env:           d.sessionEnv(info.ID, portBase),
		RolePrompt:    r.Prompt,
		AllowedTools:  r.AllowTools,
		DeniedTools:   r.DenyTools,
		Model:         r.Model,
		DenyRead:      d.policyFor(info).DenyRead,
		Instructions:  agentInstructions,
	}
	caps := a.Capabilities()
	if first && caps.InitialPrompt {
		spec.Task = info.Task
	}
	launch, err := a.Prepare(spec)
	if err != nil {
		d.ports.Release(info.ID)
		d.launchFailed(s, a, err)
		return
	}

	s.mu.Lock()
	s.stopping = false
	s.gate = nil // rules may have changed since the last launch
	if first && caps.Hooks && !caps.InitialPrompt && strings.TrimSpace(info.Task) != "" {
		s.taskPending = true
	}
	s.mu.Unlock()

	_, err = d.startPane(s, core.PaneAgent, launch, cols, rows, func(p *pane) {
		d.update(s, func(c *core.Session) {
			c.AgentPane = p.id
			c.State, c.StateDetail = core.StateStarting, ""
			c.Unseen = false
		})
	})
	if err != nil {
		d.ports.Release(info.ID)
		d.launchFailed(s, a, err)
	}
}

// launchFailed puts the session in the error state with an explanation.
func (d *Daemon) launchFailed(s *session, a agent.Adapter, err error) {
	name := s.snap().Agent
	if a != nil {
		name = a.DisplayName()
	}
	detail := fmt.Sprintf("%s could not be started: %s", name, trimPrefixErr(err))
	if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "not found") {
		if !strings.Contains(detail, "install") {
			detail += ". Install it and make sure it is on your PATH, then restart the session."
		}
	}
	d.log.Warn("agent launch failed", "session", s.id, "err", err)
	d.update(s, func(c *core.Session) {
		c.State, c.StateDetail = core.StateError, detail
		c.AgentPane = ""
	})
}

// trimPrefixErr drops a leading "package: " from an error message.
func trimPrefixErr(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i > 0 && i < 20 && !strings.ContainsAny(msg[:i], " /\\") {
		return msg[i+2:]
	}
	return msg
}
