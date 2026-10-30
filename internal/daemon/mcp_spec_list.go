package daemon

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/spec"
)

// mcpSpecList lists the specs the session can see, as JSON.
func (d *Daemon) mcpSpecList(ctx context.Context, s *session, _ json.RawMessage) (string, error) {
	seen := map[string]protocol.SpecInfo{}
	for _, root := range d.specRoots(ctx, s) {
		infos, err := spec.List(root)
		if err != nil {
			return "", err
		}
		for _, in := range infos {
			if _, dup := seen[in.Slug]; dup {
				continue // the worktree's copy wins
			}
			seen[in.Slug] = protocol.SpecInfo{
				Slug: in.Slug, Title: in.Title,
				TasksDone: in.TasksDone, TasksTotal: in.TasksTotal, CriteriaTotal: in.CriteriaTotal,
			}
		}
	}
	out := make([]protocol.SpecInfo, 0, len(seen))
	for _, in := range seen {
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return toJSONText(out)
}
