package clientbackend

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func pref(id core.PaneID) protocol.PaneRef { return protocol.PaneRef{PaneID: id} }

// OpenShell opens a shell pane in the session's worktree.
func (b *Backend) OpenShell(ctx context.Context, id core.SessionID, cols, rows int) (core.Pane, error) {
	var out core.Pane
	err := b.call(ctx, protocol.MethodPaneOpen, protocol.PaneOpenParams{SessionID: id, Cols: cols, Rows: rows}, &out)
	return out, err
}

// ClosePane closes a pane.
func (b *Backend) ClosePane(ctx context.Context, id core.PaneID) error {
	return b.call(ctx, protocol.MethodPaneClose, pref(id), nil)
}

// PaneInput sends bytes to the pane's process.
func (b *Backend) PaneInput(ctx context.Context, id core.PaneID, data []byte) error {
	return b.call(ctx, protocol.MethodPaneInput, protocol.PaneInputParams{PaneID: id, Data: data}, nil)
}

// PaneResize resizes the pane's terminal.
func (b *Backend) PaneResize(ctx context.Context, id core.PaneID, cols, rows int) error {
	return b.call(ctx, protocol.MethodPaneResize, protocol.PaneResizeParams{PaneID: id, Cols: cols, Rows: rows}, nil)
}

// PaneSnapshot returns the pane's full current screen.
func (b *Backend) PaneSnapshot(ctx context.Context, id core.PaneID) (protocol.PaneFrame, error) {
	var out protocol.PaneFrame
	err := b.call(ctx, protocol.MethodPaneSnapshot, pref(id), &out)
	return out, err
}
