package daemon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// mcpSessionMessage types a steer message into another session of the same
// workspace, naming the sender.
func (d *Daemon) mcpSessionMessage(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.SessionMessageArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	text := clipText(a.Text, maxBody)
	if text == "" {
		return "", badRequest("text is required")
	}
	targets, err := d.resolveSessions(s, []string{a.Session})
	if err != nil {
		return "", err
	}
	target := targets[0]
	if target == s {
		return "", badRequest("a session cannot message itself")
	}
	msg := fmt.Sprintf("Message from session %s: %s", s.snap().Slug, text)
	if err := d.steer(ctx, target, msg, a.Interrupt); err != nil {
		return "", err
	}
	return "message sent to " + target.snap().Slug, nil
}
