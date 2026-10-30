package daemon

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hAgentList lists the agent adapters registered with the daemon and whether
// each one's program is installed.
func (d *Daemon) hAgentList(_ context.Context, _ *conn, _ protocol.Empty) (any, error) {
	def := firstNonEmpty(d.settings.DefaultAgent, defaultAgent)
	names := agent.Names()
	out := make([]protocol.AgentInfo, 0, len(names))
	for _, n := range names {
		a, ok := agent.Get(n)
		if !ok {
			continue
		}
		found, detail := agent.Detect(a)
		out = append(out, protocol.AgentInfo{
			Name: a.Name(), DisplayName: a.DisplayName(), Available: found, Detail: detail,
			Default: a.Name() == def,
		})
	}
	return out, nil
}
