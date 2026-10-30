package pane

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Msg is implemented by the messages a Pane sends to itself. The owner
// forwards any Msg to Dest().Update.
type Msg interface{ Dest() *Pane }

// SnapshotMsg carries the result of a snapshot fetch.
type SnapshotMsg struct {
	Pane  *Pane
	ID    core.PaneID
	Frame protocol.PaneFrame
	Err   error
}

// Dest returns the pane the message belongs to.
func (m SnapshotMsg) Dest() *Pane { return m.Pane }

func (p *Pane) snapshotCmd() tea.Cmd {
	p.sawFrame = false
	id, be := p.id, p.be
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		f, err := be.PaneSnapshot(ctx, id)
		return SnapshotMsg{Pane: p, ID: id, Frame: f, Err: err}
	}
}

// handleSnapshot applies a fetched screen, unless a newer full frame already
// arrived, and re-fetches once if incremental frames raced with the fetch.
func (p *Pane) handleSnapshot(m SnapshotMsg) tea.Cmd {
	if m.ID != p.id || m.Err != nil {
		return nil
	}
	var cmds []tea.Cmd
	if !p.gotFull {
		f := m.Frame
		f.PaneID, f.Full = p.id, true
		raced := p.sawFrame
		p.gotFull = true
		p.apply(f)
		if raced {
			p.gotFull = false
			cmds = append(cmds, p.snapshotCmd())
		}
	}
	if p.wantCols > 0 && (p.wantCols != p.cols || p.wantRows != p.nrows) {
		cmds = append(cmds, p.sendResize())
	}
	return tea.Batch(cmds...)
}
