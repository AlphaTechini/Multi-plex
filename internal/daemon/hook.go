package daemon

import (
	"context"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// permissionPrefix starts the Message of a notification that means the agent
// CLI is showing its own permission prompt. It is the adapter convention of
// claudecode.PermissionPrefix (a test keeps the two equal).
const permissionPrefix = "permission:"

// turnEndTimeout bounds the work done after a turn ends.
const turnEndTimeout = 2 * time.Minute

// hHook handles one agent CLI hook call from `mpx hook`. Each call runs on
// its own goroutine, so a pre-tool call waiting for an approval blocks only
// itself. The connection's context ends if the hook process goes away, which
// cancels the wait and denies the request.
func (d *Daemon) hHook(ctx context.Context, _ *conn, p protocol.HookParams) (any, error) {
	start := time.Now()
	ev := p.Event
	s, err := d.getSession(p.SessionID)
	if err != nil {
		if ev.Kind == agent.HookPreTool {
			return agent.HookResponse{
				Decision: agent.Deny,
				Reason:   "Blocked by Multiplex: this agent is not part of a known Multiplex session.",
			}, nil
		}
		return agent.HookResponse{}, nil
	}

	var resp agent.HookResponse
	switch ev.Kind {
	case agent.HookPreTool:
		resp = d.hookPreTool(ctx, s, ev)
	case agent.HookPostTool:
		d.hookPostTool(s, ev)
	case agent.HookNotification:
		d.hookNotification(s, ev)
	case agent.HookStop:
		d.hookStop(s)
	case agent.HookPromptSubmit:
		d.markWorking(s)
	case agent.HookSessionStart:
		d.hookSessionStart(s)
	case agent.HookSessionEnd:
		d.setState(s, core.StateIdle, "agent session ended")
	default:
		return nil, badRequest("unknown hook kind %q", ev.Kind)
	}
	// Never logs the command or tool input: only the kind, tool name and timing.
	d.log.Debug("hook", "session", s.id, "kind", ev.Kind, "tool", ev.ToolName, "decision", resp.Decision, "took", time.Since(start))
	return resp, nil
}

// markWorking moves the session to working, unless an approval is waiting.
// A prompt or tool call after a quiet period also covers a user interrupt,
// which produces no Stop event.
func (d *Daemon) markWorking(s *session) {
	d.update(s, func(c *core.Session) {
		if s.blockCount > 0 || c.State == core.StateError {
			return
		}
		c.State, c.StateDetail, c.Unseen = core.StateWorking, "", false
	})
}

// hookPostTool records that a tool ran: the agent is working again, written
// files make the next checkpoint worthwhile, and edits feed follow mode.
func (d *Daemon) hookPostTool(s *session, ev agent.HookEvent) {
	d.markWorking(s)
	writes := ev.ToolClass == agent.ToolWrite || isShell(ev)
	if writes {
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
	}
	if len(ev.Edits) > 0 {
		root := s.snap().WorktreePath
		now := d.now()
		for _, e := range ev.Edits {
			d.publish(protocol.TopicEdit, core.EditEvent{
				SessionID: s.id,
				Path:      relativeTo(root, e.Path),
				StartLine: e.StartLine,
				EndLine:   e.EndLine,
				At:        now,
			})
		}
	}
	if writes {
		d.scheduleDiffChanged(s)
	}
}

// hookNotification maps a notification to a state. A permission prompt is the
// immediate "blocked" signal. An idle notification means the agent finished
// and waits for input. Anything else keeps the current state.
func (d *Daemon) hookNotification(s *session, ev agent.HookEvent) {
	msg := ev.Message
	switch {
	case strings.HasPrefix(msg, permissionPrefix):
		d.setState(s, core.StateBlocked, "waiting for permission")
	case isIdleNotification(msg):
		d.update(s, func(c *core.Session) {
			if c.State == core.StateWorking || c.State == core.StateStarting || c.State == core.StateBlocked {
				c.State, c.StateDetail = core.StateDone, ""
			}
		})
	}
}

// isIdleNotification reports whether a notification says the agent is idle
// and waiting for the user's input.
func isIdleNotification(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "waiting for your input") ||
		strings.Contains(m, "waiting for input") ||
		strings.Contains(m, "is idle")
}

// hookStop marks the turn done and starts the work that follows a turn:
// a checkpoint, conflict prediction and a diff refresh.
func (d *Daemon) hookStop(s *session) {
	d.update(s, func(c *core.Session) {
		c.State, c.StateDetail, c.Unseen = core.StateDone, "", true
	})
	d.background(func() { d.afterTurn(s) })
}

// afterTurn runs off the request goroutine after a turn ends.
func (d *Daemon) afterTurn(s *session) {
	ctx, cancel := context.WithTimeout(d.ctx, turnEndTimeout)
	defer cancel()
	if _, err := d.takeCheckpoint(ctx, s, "turn end"); err != nil {
		d.log.Warn("turn-end checkpoint failed", "session", s.id, "err", err)
	}
	d.announceDiff(s)
	d.predictConflicts(ctx, s)
}

// hookSessionStart types the task into a hook-capable agent when it has
// started, once. The short delay lets its input box appear.
func (d *Daemon) hookSessionStart(s *session) {
	s.mu.Lock()
	typeIt := s.taskPending && !s.taskTyped
	if typeIt {
		s.taskTyped = true
	}
	info := s.info
	s.mu.Unlock()
	if !typeIt {
		return
	}
	a, err := adapterFor(info)
	if err != nil {
		return
	}
	time.AfterFunc(taskTypeDelay, func() {
		if d.ctx.Err() != nil {
			return
		}
		if err := d.writeAgent(s, a.TypeMessage(info.Task)); err != nil {
			d.log.Warn("could not type the task", "session", s.id, "err", err)
		}
	})
}

// taskTypeDelay is how long after SessionStart the task is typed.
const taskTypeDelay = 500 * time.Millisecond

// isShell reports whether the event is a shell tool call.
func isShell(ev agent.HookEvent) bool {
	return ev.ToolClass == agent.ToolShell || (ev.ToolClass == "" && ev.Command != "")
}
