package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

type fakeCaller struct {
	mu    sync.Mutex
	calls []protocol.MCPParams
	res   protocol.MCPResult
	err   error
}

func (f *fakeCaller) MCP(_ context.Context, p protocol.MCPParams) (protocol.MCPResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, p)
	return f.res, f.err
}

// connect starts the server in memory and returns a connected MCP client.
func connect(t *testing.T, session core.SessionID, call Caller) *mcp.ClientSession {
	t.Helper()
	ctx := t.Context()
	st, ct := mcp.NewInMemoryTransports()
	srv := New("test", session, call)
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestToolsListed(t *testing.T) {
	cs := connect(t, "ses_1", &fakeCaller{})
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	byName := map[string]*mcp.Tool{}
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
		byName[tl.Name] = tl
		if tl.Description == "" {
			t.Errorf("tool %s has no description", tl.Name)
		}
	}
	sort.Strings(names)
	want := []string{"criterion_verify", "note_to_user", "report_progress", "session_message", "session_spawn", "session_status", "session_wait", "spec_get", "spec_list", "task_update"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("tools = %v, want %v", names, want)
	}
	for _, n := range names {
		if strings.Contains(n, "approv") {
			t.Errorf("tool %s looks like an approval tool", n)
		}
	}
	// Field descriptions reach the agent.
	b, _ := json.Marshal(byName["note_to_user"].InputSchema)
	if !strings.Contains(string(b), "decision-needed") {
		t.Errorf("note_to_user schema lacks field descriptions: %s", b)
	}
}

func TestToolCallForwards(t *testing.T) {
	fc := &fakeCaller{res: protocol.MCPResult{Text: "recorded"}}
	cs := connect(t, "ses_42", fc)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "report_progress",
		Arguments: map[string]any{"done": []string{"a", "b"}, "next": "c"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || len(res.Content) != 1 || res.Content[0].(*mcp.TextContent).Text != "recorded" {
		t.Errorf("result = %+v", res)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("calls = %d", len(fc.calls))
	}
	c := fc.calls[0]
	if c.SessionID != "ses_42" || c.Tool != "report_progress" {
		t.Errorf("call = %+v", c)
	}
	var args protocol.ReportProgressArgs
	if err := json.Unmarshal(c.Args, &args); err != nil || len(args.Done) != 2 || args.Next != "c" {
		t.Errorf("args = %s (%v)", c.Args, err)
	}
}

func TestEmptyArgsTool(t *testing.T) {
	fc := &fakeCaller{res: protocol.MCPResult{Text: "[]"}}
	cs := connect(t, "s", fc)
	if _, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "spec_list", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if len(fc.calls) != 1 || fc.calls[0].Tool != "spec_list" {
		t.Errorf("calls = %+v", fc.calls)
	}
}

func TestDaemonErrorsReachTheAgent(t *testing.T) {
	fc := &fakeCaller{res: protocol.MCPResult{Text: "no such spec", IsError: true}}
	cs := connect(t, "s", fc)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "spec_get", Arguments: map[string]any{"slug": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || res.Content[0].(*mcp.TextContent).Text != "no such spec" {
		t.Errorf("result = %+v", res)
	}

	fc2 := &fakeCaller{err: errors.New("dial unix: connect: no such file")}
	cs2 := connect(t, "s", fc2)
	res, err = cs2.CallTool(t.Context(), &mcp.CallToolParams{Name: "note_to_user", Arguments: map[string]any{"title": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "cannot reach the Multiplex daemon") {
		t.Errorf("result = %+v", res)
	}
}

func TestNoSessionID(t *testing.T) {
	fc := &fakeCaller{}
	cs := connect(t, "", fc)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "note_to_user", Arguments: map[string]any{"title": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "MPX_SESSION_ID") || len(fc.calls) != 0 {
		t.Errorf("result = %+v, calls = %d", res, len(fc.calls))
	}
}

func TestInvalidArgumentsRejectedBeforeForwarding(t *testing.T) {
	fc := &fakeCaller{}
	cs := connect(t, "s", fc)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "task_update", Arguments: map[string]any{"slug": 7}})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("invalid arguments were accepted")
	}
	if len(fc.calls) != 0 {
		t.Error("invalid call was forwarded")
	}
}
