package fake

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// pushFrameLocked emits a pane.frame for p (only if rows changed, or always
// for a full frame). The caller holds b.mu.
func (b *Backend) pushFrameLocked(p *fakePane, full bool) {
	if f, ok := p.frame(full); ok {
		b.emit(protocol.TopicPaneFrame, f)
	}
}

// OpenShell opens a scripted shell pane in the session's worktree.
func (b *Backend) OpenShell(ctx context.Context, id core.SessionID, cols, rows int) (core.Pane, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("OpenShell")
	s := b.session(id)
	if s == nil {
		return core.Pane{}, notFound("session")
	}
	pid := core.PaneID(b.nextID("pane"))
	p := newShellPane(pid, *s, max(cols, 10), max(rows, 3))
	b.panes[pid] = p
	return p.info(), ctxErr(ctx)
}

// ClosePane forgets a pane.
func (b *Backend) ClosePane(ctx context.Context, id core.PaneID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("ClosePane")
	delete(b.panes, id)
	return ctxErr(ctx)
}

// PaneInput feeds typed bytes to the scripted program in the pane.
func (b *Backend) PaneInput(ctx context.Context, id core.PaneID, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("PaneInput")
	p := b.panes[id]
	if p == nil {
		return notFound("pane")
	}
	if p.exited {
		return nil
	}
	lines := p.typeInput(data)
	switch p.kind {
	case core.PaneShell:
		for _, l := range lines {
			p.runShell(l)
		}
	default:
		if s := b.session(p.session); s != nil {
			for _, l := range lines {
				b.userMessageLocked(s, l)
			}
		}
	}
	b.pushFrameLocked(p, false)
	return ctxErr(ctx)
}

// PaneResize changes the pane's size and sends a full frame.
func (b *Backend) PaneResize(ctx context.Context, id core.PaneID, cols, rows int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("PaneResize")
	p := b.panes[id]
	if p == nil {
		return notFound("pane")
	}
	if cols > 0 && rows > 0 && (cols != p.cols || rows != p.rows) {
		p.cols, p.rows = cols, rows
		b.pushFrameLocked(p, true)
	}
	return ctxErr(ctx)
}

// PaneSnapshot returns the pane's full screen.
func (b *Backend) PaneSnapshot(ctx context.Context, id core.PaneID) (protocol.PaneFrame, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("PaneSnapshot")
	p := b.panes[id]
	if p == nil {
		return protocol.PaneFrame{}, notFound("pane")
	}
	f, _ := p.frame(true)
	return f, ctxErr(ctx)
}
