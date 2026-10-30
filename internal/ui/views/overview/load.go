package overview

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
)

// rpcTimeout bounds every backend call the view makes.
const rpcTimeout = 5 * time.Second

// loadedMsg is the result of loadCmd.
type loadedMsg struct {
	seq    int
	id     core.SessionID
	detail protocol.SessionDetail
	names  map[core.SessionID]string
	spec   *protocol.SpecInfo
	keep   bool // specs were not fetched: keep the previous spec
	err    error
	at     time.Time
}

// loadCmd fetches the session detail. It also fetches the session list (for
// the names in conflict rows) when there are conflicts, and the spec list
// (to find the spec this session's task belongs to) when withSpecs is set.
func loadCmd(be backend.Backend, ws core.WorkspaceID, id core.SessionID, seq int, withSpecs bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()
		out := loadedMsg{seq: seq, id: id, keep: !withSpecs}
		d, err := be.Session(ctx, id)
		if err != nil {
			out.err = err
			return out
		}
		out.detail = d
		if len(d.Conflicts) > 0 {
			if list, err := be.Sessions(ctx, ws); err == nil {
				out.names = make(map[core.SessionID]string, len(list))
				for _, s := range list {
					out.names[s.ID] = s.Slug
				}
			}
		}
		if withSpecs {
			if specs, err := be.Specs(ctx, ws); err == nil {
				out.spec = matchSpec(specs, d.Session)
			}
		}
		out.at = time.Now()
		return out
	}
}

// matchSpec finds the spec a session's task belongs to by looking for the
// spec's slug in the task text (best effort). The longest slug wins so
// "token-refresh-v2" beats "token-refresh".
func matchSpec(specs []protocol.SpecInfo, s core.Session) *protocol.SpecInfo {
	task := strings.ToLower(s.Task)
	var best *protocol.SpecInfo
	for i := range specs {
		slug := strings.ToLower(specs[i].Slug)
		if slug == "" || !strings.Contains(task, slug) {
			continue
		}
		if best == nil || len(slug) > len(best.Slug) {
			best = &specs[i]
		}
	}
	if best != nil {
		c := *best
		return &c
	}
	return nil
}
