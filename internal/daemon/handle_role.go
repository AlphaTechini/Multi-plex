package daemon

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/role"
)

// hRoleList lists the roles visible to a workspace: built-in, global and
// workspace roles, with workspace over global over built-in by name.
func (d *Daemon) hRoleList(ctx context.Context, _ *conn, p protocol.WorkspaceRef) (any, error) {
	ws, err := d.workspace(ctx, p.WorkspaceID)
	if err != nil {
		return nil, err
	}
	globalDir := filepath.Join(d.paths.ConfigDir, "roles")
	roles, rerr := role.Load(ws.Root, globalDir)
	if rerr != nil {
		d.log.Warn("role files ignored", "err", rerr)
	}
	def := firstNonEmpty(d.settings.DefaultRole, defaultRole)
	out := make([]protocol.RoleInfo, 0, len(roles))
	for _, r := range roles {
		out = append(out, protocol.RoleInfo{
			Name: r.Name, Description: r.Description, Source: roleSource(r, globalDir),
			Default: r.Name == def,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// roleSource names where a role came from.
func roleSource(r role.Role, globalDir string) string {
	switch {
	case r.Source == "builtin":
		return "builtin"
	case strings.HasPrefix(filepath.Clean(r.Source), filepath.Clean(globalDir)+string(filepath.Separator)):
		return "global"
	default:
		return "workspace"
	}
}
