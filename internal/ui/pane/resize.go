package pane

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
)

type resizeMsg struct {
	pane *Pane
	seq  int
}

// Dest returns the pane the message belongs to.
func (m resizeMsg) Dest() *Pane { return m.pane }

// Resize records the size the pane is drawn at. The daemon is told after
// ResizeDebounce without further changes, so dragging a divider or resizing
// the window sends one call. The returned command is nil when nothing
// changed. Route the resulting Msg to Update.
func (p *Pane) Resize(cols, rows int) tea.Cmd {
	if cols <= 0 || rows <= 0 || (cols == p.wantCols && rows == p.wantRows) {
		return nil
	}
	p.wantCols, p.wantRows = cols, rows
	p.resizeSeq++
	if p.id == "" {
		return nil
	}
	seq := p.resizeSeq
	return tea.Tick(ResizeDebounce, func(time.Time) tea.Msg { return resizeMsg{pane: p, seq: seq} })
}

func (p *Pane) handleResize(m resizeMsg) tea.Cmd {
	if m.seq != p.resizeSeq || p.id == "" {
		return nil
	}
	if p.wantCols == p.sentCols && p.wantRows == p.sentRows {
		return nil
	}
	return p.sendResize()
}

func (p *Pane) sendResize() tea.Cmd {
	id, be := p.id, p.be
	cols, rows := p.wantCols, p.wantRows
	p.sentCols, p.sentRows = cols, rows
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = be.PaneResize(ctx, id, cols, rows)
		return nil
	}
}
