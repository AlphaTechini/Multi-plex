package mcpserver

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// New builds the MCP server for one session. Every tool forwards to call.
// sessionID is the session the agent belongs to (from MPX_SESSION_ID).
func New(version string, sessionID core.SessionID, call Caller) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "multiplex", Title: "Multiplex", Version: version}, nil)
	f := &toolFn{session: sessionID, call: call}

	add(s, f, protocol.ToolReportProgress,
		"Report a structured summary of your progress. Call it at the end of every turn and at milestones: what is done, files changed, decisions and open questions.",
		protocol.ReportProgressArgs{})
	add(s, f, protocol.ToolNoteToUser,
		"Show the user an insight card for something they should know that does not need an answer right now. Use severity decision-needed when it does.",
		protocol.NoteToUserArgs{})
	add(s, f, protocol.ToolSpecList,
		"List the specs (requirements, design, tasks) available in this workspace with their progress.",
		protocol.SpecListArgs{})
	add(s, f, protocol.ToolSpecGet,
		"Read a spec's requirements, design and tasks, or one requirement or acceptance criterion by ID (for example R1 or R1.2).",
		protocol.SpecGetArgs{})
	add(s, f, protocol.ToolTaskUpdate,
		"Mark a spec task open, in-progress, done or blocked, with a short note. The change is made in your worktree so the user can review it in the diff.",
		protocol.TaskUpdateArgs{})
	add(s, f, protocol.ToolCriterionVerify,
		"Record that an acceptance criterion is met, with evidence such as the test name or file and lines.",
		protocol.CriterionVerifyArgs{})
	add(s, f, protocol.ToolSessionSpawn,
		"Start another agent session in this workspace with a role and a task. Returns its session ID. For orchestrators.",
		protocol.SessionSpawnArgs{})
	add(s, f, protocol.ToolSessionStatus,
		"Get the state, latest summary and open notes of other sessions in this workspace.",
		protocol.SessionStatusArgs{})
	add(s, f, protocol.ToolSessionWait,
		"Wait until the named sessions are done, blocked, idle or in error, or until the timeout. Returns their status.",
		protocol.SessionWaitArgs{})
	add(s, f, protocol.ToolSessionMessage,
		"Send a steer message to another session in this workspace.",
		protocol.SessionMessageArgs{})
	return s
}

// Run serves the tools over t until it closes or ctx ends.
func Run(ctx context.Context, srv *mcp.Server, t mcp.Transport) error {
	err := srv.Run(ctx, t)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// RunStdio serves the tools over the process's standard input and output,
// the way an agent CLI launches `mpx mcp`.
func RunStdio(ctx context.Context, srv *mcp.Server) error {
	return Run(ctx, srv, &mcp.StdioTransport{})
}

// toolFn forwards tool calls of one session.
type toolFn struct {
	session core.SessionID
	call    Caller
}

// add registers a tool whose input type is the type of sample.
func add[In any](s *mcp.Server, f *toolFn, name, desc string, _ In) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		return f.forward(ctx, name, in)
	})
}

// forward sends one call to the daemon and turns its answer into a tool result.
func (f *toolFn) forward(ctx context.Context, tool string, in any) (*mcp.CallToolResult, any, error) {
	if f.session == "" {
		return nil, nil, errors.New("this agent was not started by Multiplex (MPX_SESSION_ID is not set), so the Multiplex tools are unavailable")
	}
	args, err := json.Marshal(in)
	if err != nil {
		return nil, nil, err
	}
	res, err := f.call.MCP(ctx, protocol.MCPParams{SessionID: f.session, Tool: tool, Args: args})
	if err != nil {
		return nil, nil, errors.New("cannot reach the Multiplex daemon: " + err.Error())
	}
	return &mcp.CallToolResult{
		IsError: res.IsError,
		Content: []mcp.Content{&mcp.TextContent{Text: res.Text}},
	}, nil, nil
}
