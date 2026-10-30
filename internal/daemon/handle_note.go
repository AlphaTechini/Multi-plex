package daemon

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hNoteList lists a session's insight cards.
func (d *Daemon) hNoteList(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	if _, err := d.getSession(p.SessionID); err != nil {
		return nil, err
	}
	notes, err := d.store.ListNotes(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	if notes == nil {
		notes = []core.Note{}
	}
	return notes, nil
}

// hNoteAct performs a card action (docs/08-insights-and-steering.md).
func (d *Daemon) hNoteAct(ctx context.Context, _ *conn, p protocol.NoteActParams) (any, error) {
	s, n, err := d.findNote(ctx, p.NoteID)
	if err != nil {
		return nil, err
	}
	a, err := adapterFor(s.snap())
	if err != nil {
		return nil, err
	}
	switch p.Action {
	case core.NoteDismiss:
		if err := d.store.DismissNote(ctx, n.ID); err != nil {
			return nil, err
		}
	case core.NoteAsk:
		if p.Text == "" {
			return nil, badRequest("a question is required")
		}
		card := fmt.Sprintf("Re: card #%d %s: %s\n%s", n.Number, n.Title, n.Body, p.Text)
		b, ok := a.SideQuestion(card)
		if !ok {
			return nil, badRequest("%s does not support side questions", a.DisplayName())
		}
		if err := d.writeAgent(s, b); err != nil {
			return nil, err
		}
	case core.NoteSteer, core.NoteInterruptSteer:
		if p.Text == "" {
			return nil, badRequest("a message is required")
		}
		msg := fmt.Sprintf("Re card #%d (%s): %s", n.Number, n.Title, p.Text)
		if err := d.steer(ctx, s, msg, p.Action == core.NoteInterruptSteer); err != nil {
			return nil, err
		}
	default:
		return nil, badRequest("unknown note action %q", p.Action)
	}
	return protocol.Empty{}, nil
}

// findNote locates a note by ID across the live sessions.
func (d *Daemon) findNote(ctx context.Context, id string) (*session, core.Note, error) {
	for _, s := range d.sessionsOf("") {
		notes, err := d.store.ListNotes(ctx, s.id)
		if err != nil {
			return nil, core.Note{}, err
		}
		for _, n := range notes {
			if n.ID == id {
				return s, n, nil
			}
		}
	}
	return nil, core.Note{}, notFound("no such note %q", id)
}

// hSummaryGet returns a session's latest progress report.
func (d *Daemon) hSummaryGet(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	if _, err := d.getSession(p.SessionID); err != nil {
		return nil, err
	}
	return d.store.LatestSummary(ctx, p.SessionID)
}
