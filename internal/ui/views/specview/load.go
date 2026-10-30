package specview

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
)

// rpcTimeout bounds every backend call the view makes.
const rpcTimeout = 5 * time.Second

// specsMsg is the result of specsCmd.
type specsMsg struct {
	specs []protocol.SpecInfo
	err   error
}

// docMsg is the result of docCmd.
type docMsg struct {
	slug string
	doc  protocol.SpecDoc
	err  error
}

// specsCmd lists the workspace's specs.
func specsCmd(be backend.Backend, ws core.WorkspaceID) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()
		s, err := be.Specs(ctx, ws)
		return specsMsg{specs: s, err: err}
	}
}

// docCmd reads one spec's three documents.
func docCmd(be backend.Backend, ws core.WorkspaceID, slug string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()
		d, err := be.Spec(ctx, ws, slug)
		return docMsg{slug: slug, doc: d, err: err}
	}
}
