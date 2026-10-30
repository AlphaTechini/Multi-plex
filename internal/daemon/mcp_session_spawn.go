package daemon

import (
	"context"
	"encoding/json"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// maxSessionsPerWorkspace stops a runaway orchestrator from filling the disk
// with worktrees.
const maxSessionsPerWorkspace = 50

// mcpSessionSpawn starts a session in the caller's workspace and returns its ID.
func (d *Daemon) mcpSessionSpawn(_ context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.SessionSpawnArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	if clipText(a.Task, maxBody) == "" {
		return "", badRequest("task is required")
	}
	info := s.snap()
	if len(d.sessionsOf(info.WorkspaceID)) >= maxSessionsPerWorkspace {
		return "", badRequest("this workspace already has %d sessions; remove some first", maxSessionsPerWorkspace)
	}
	child, err := d.createSession(protocol.SessionCreateParams{
		WorkspaceID: info.WorkspaceID,
		Task:        clipText(a.Task, maxBody),
		Name:        clipText(a.Name, maxTitle),
		Role:        a.Role,
		Agent:       a.Agent,
		BaseBranch:  a.BaseBranch,
	})
	if err != nil {
		return "", err
	}
	ci := child.snap()
	d.log.Info("session spawned by agent", "parent", info.ID, "child", ci.ID)
	return toJSONText(map[string]any{
		"session_id": ci.ID, "slug": ci.Slug, "branch": ci.Branch, "role": ci.Role, "state": ci.State,
	})
}
