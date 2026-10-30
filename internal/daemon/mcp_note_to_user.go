package daemon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// mcpNoteToUser creates an insight card. It does not change the session's
// state: the card shows as a badge, even for decision-needed.
func (d *Daemon) mcpNoteToUser(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.NoteToUserArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	title := clipText(a.Title, maxTitle)
	if title == "" {
		return "", badRequest("title is required")
	}
	sev := core.Severity(a.Severity)
	switch sev {
	case "":
		sev = core.SeverityInfo
	case core.SeverityInfo, core.SeverityWarning, core.SeverityDecisionNeeded:
	default:
		return "", badRequest("severity must be info, warning or decision-needed, got %q", a.Severity)
	}
	n, err := d.store.AddNote(ctx, core.Note{
		ID:        core.NewID("note"),
		SessionID: s.id,
		Title:     title,
		Body:      clipText(a.Body, maxBody),
		Severity:  sev,
		Refs:      clipList(a.Refs, maxItem),
		At:        d.now(),
	})
	if err != nil {
		return "", err
	}
	d.publish(protocol.TopicNote, n)
	return fmt.Sprintf("card #%d created", n.Number), nil
}
