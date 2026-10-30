package daemon

import (
	"context"
	"encoding/json"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// mcpSessionStatus returns state, latest summary and notes of sessions in the
// caller's workspace.
func (d *Daemon) mcpSessionStatus(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.SessionStatusArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	targets, err := d.resolveSessions(s, a.Sessions)
	if err != nil {
		return "", err
	}
	out := make([]sessionBrief, len(targets))
	for i, t := range targets {
		out[i] = d.briefOf(ctx, t)
	}
	return toJSONText(out)
}
