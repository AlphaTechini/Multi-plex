package daemon

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/role"
)

// Defaults for new sessions.
const (
	defaultRole   = "implementer"
	defaultAgent  = "claude-code"
	branchPrefix  = "mpx/"
	disabledPush  = "multiplex-push-broker:disabled"
	createTimeout = 2 * time.Minute
)

// hSessionCreate creates a session and starts its agent.
func (d *Daemon) hSessionCreate(_ context.Context, _ *conn, p protocol.SessionCreateParams) (any, error) {
	s, err := d.createSession(p)
	if err != nil {
		return nil, err
	}
	return s.snap(), nil
}

// createSession makes a worktree and branch for a new session and launches
// its agent (docs/05-sessions-and-worktrees.md "Session lifecycle"). It
// returns an error only when the session could not be set up at all; an agent
// that fails to start leaves a session in the error state.
func (d *Daemon) createSession(p protocol.SessionCreateParams) (*session, error) {
	ctx, cancel := context.WithTimeout(d.ctx, createTimeout)
	defer cancel()

	ws, err := d.workspace(ctx, p.WorkspaceID)
	if err != nil {
		return nil, err
	}
	roleName := firstNonEmpty(p.Role, d.settings.DefaultRole, defaultRole)
	roles, rerr := role.Load(ws.Root, filepath.Join(d.paths.ConfigDir, "roles"))
	if rerr != nil {
		d.log.Warn("role files ignored", "err", rerr)
	}
	r, ok := roles[roleName]
	if !ok {
		names := make([]string, 0, len(roles))
		for n := range roles {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, badRequest("unknown role %q (available: %s)", roleName, strings.Join(names, ", "))
	}
	agentName := p.Agent
	if agentName == "" && r.Source != "builtin" {
		agentName = r.Agent
	}
	agentName = firstNonEmpty(agentName, d.settings.DefaultAgent, defaultAgent)
	if _, err := adapterFor(core.Session{Agent: agentName}); err != nil {
		return nil, err
	}

	base := p.BaseBranch
	if base == "" {
		if base, err = d.vcs.CurrentBranch(ctx, ws.Root); err != nil || base == "" {
			base = "HEAD"
		}
	}

	d.createMu.Lock()
	info, err := d.addWorktree(ctx, ws, p, base)
	if err != nil {
		d.createMu.Unlock()
		return nil, err
	}
	info.Role, info.Agent = roleName, agentName
	s := newSession(info)
	d.mu.Lock()
	d.sessions[info.ID] = s
	d.mu.Unlock()
	d.createMu.Unlock()

	if err := d.store.PutSession(ctx, info); err != nil {
		d.log.Error("save new session", "session", info.ID, "err", err)
	}
	d.publish(protocol.TopicSession, info)
	d.notifyState()

	d.launchAgent(ctx, s, ws, p.Cols, p.Rows, true)
	return s, nil
}

// addWorktree picks a unique slug, creates the branch and worktree, disables
// the worktree's push URL when the repository has an origin remote
// (docs/04-safety.md Layer 3) and returns the new session record. The caller
// holds createMu.
func (d *Daemon) addWorktree(ctx context.Context, ws core.Workspace, p protocol.SessionCreateParams, base string) (core.Session, error) {
	slugBase := slugify(firstNonEmpty(p.Name, p.Task))
	taken := func(slug string) bool {
		d.mu.RLock()
		defer d.mu.RUnlock()
		for _, s := range d.sessions {
			if i := s.snap(); i.WorkspaceID == ws.ID && i.Slug == slug {
				return true
			}
		}
		return false
	}

	var info core.Session
	var lastErr error
	tried := map[string]bool{}
	pick := func() string {
		slug := uniqueSlug(slugBase, func(c string) bool { return taken(c) || tried[c] })
		tried[slug] = true
		return slug
	}
	slug := pick()
	for attempt := 0; attempt < 20; attempt++ {
		path := filepath.Join(d.paths.WorktreesDir, string(ws.ID), slug)
		branch := branchPrefix + slug
		if _, statErr := os.Stat(path); statErr == nil {
			lastErr = badRequest("worktree path %s already exists", path)
		} else if err := d.vcs.WorktreeAdd(ctx, ws.Root, path, branch, base); err != nil {
			lastErr = err
			if !strings.Contains(err.Error(), "already exists") {
				return core.Session{}, badRequest("cannot create the worktree: %v", err)
			}
		} else {
			now := d.now()
			info = core.Session{
				ID:           core.SessionID(core.NewID("ses")),
				WorkspaceID:  ws.ID,
				Slug:         slug,
				Task:         p.Task,
				BaseBranch:   base,
				Branch:       branch,
				WorktreePath: path,
				State:        core.StateStarting,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			lastErr = nil
			break
		}
		// Branch or path left over from an earlier session: try the next slug.
		slug = pick()
	}
	if lastErr != nil {
		return core.Session{}, lastErr
	}

	if d.remotes != nil && d.remotes.HasRemote(ctx, ws.Root, "origin") {
		if err := d.vcs.SetWorktreeConfig(ctx, info.WorktreePath, "remote.origin.pushurl", disabledPush); err != nil {
			// Fail closed: without the broker URL a direct push could work.
			_ = d.vcs.WorktreeRemove(ctx, ws.Root, info.WorktreePath)
			_ = d.vcs.DeleteBranch(ctx, ws.Root, info.Branch)
			return core.Session{}, badRequest("cannot protect the worktree's push URL: %v", err)
		}
	}
	if err := os.MkdirAll(d.sessionConfigDir(info.ID), 0o700); err != nil {
		_ = d.vcs.WorktreeRemove(ctx, ws.Root, info.WorktreePath)
		_ = d.vcs.DeleteBranch(ctx, ws.Root, info.Branch)
		return core.Session{}, err
	}
	return info, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
