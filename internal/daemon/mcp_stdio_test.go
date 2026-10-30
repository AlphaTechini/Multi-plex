package daemon

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/mcpserver"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// TestMCPServerThroughDaemon runs the real `mpx mcp` server code against the
// real daemon: an MCP client calls note_to_user and report_progress, and the
// daemon publishes note.added and summary.updated.
func TestMCPServerThroughDaemon(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "mcp end to end")

	fwd := mcpserver.NewForwarder(func(ctx context.Context) (*client.Client, error) { return client.Dial(ctx, h.addr) })
	defer fwd.Close()
	srv := mcpserver.New("test", sess.ID, fwd)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(t.Context(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "0"}, nil).Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "note_to_user",
		Arguments: map[string]any{"title": "Heads up", "body": "details", "severity": "warning"},
	})
	if err != nil || res.IsError {
		t.Fatalf("note_to_user: %v %+v", err, res)
	}
	n := decode[core.Note](t, h.ev.wait(t, protocol.TopicNote, nil))
	if n.Title != "Heads up" || n.SessionID != sess.ID {
		t.Errorf("note.added = %+v", n)
	}

	res, err = cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "report_progress",
		Arguments: map[string]any{"done": []string{"wired it up"}},
	})
	if err != nil || res.IsError {
		t.Fatalf("report_progress: %v %+v", err, res)
	}
	sum := decode[core.Summary](t, h.ev.wait(t, protocol.TopicSummary, nil))
	if sum.Done[0] != "wired it up" {
		t.Errorf("summary.updated = %+v", sum)
	}

	// The forwarder redials after the daemon connection drops.
	_ = fwd.Close()
	res, err = cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "spec_list", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("spec_list after reconnect: %v %+v", err, res)
	}
}
