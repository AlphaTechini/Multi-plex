package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// mcpTool runs one MCP tool for the calling session and returns its text.
type mcpTool func(d *Daemon, ctx context.Context, s *session, args json.RawMessage) (string, error)

// mcpTools maps a tool name to its implementation. There is deliberately no
// tool to approve or deny anything: an agent can never approve a gated action.
var mcpTools = map[string]mcpTool{
	protocol.ToolReportProgress:  (*Daemon).mcpReportProgress,
	protocol.ToolNoteToUser:      (*Daemon).mcpNoteToUser,
	protocol.ToolSpecList:        (*Daemon).mcpSpecList,
	protocol.ToolSpecGet:         (*Daemon).mcpSpecGet,
	protocol.ToolTaskUpdate:      (*Daemon).mcpTaskUpdate,
	protocol.ToolCriterionVerify: (*Daemon).mcpCriterionVerify,
	protocol.ToolSessionSpawn:    (*Daemon).mcpSessionSpawn,
	protocol.ToolSessionStatus:   (*Daemon).mcpSessionStatus,
	protocol.ToolSessionWait:     (*Daemon).mcpSessionWait,
	protocol.ToolSessionMessage:  (*Daemon).mcpSessionMessage,
}

// hMCP runs one MCP tool call from `mpx mcp`. Tool failures are results with
// IsError set, so the agent sees the message; only a malformed call is a
// protocol error.
func (d *Daemon) hMCP(ctx context.Context, _ *conn, p protocol.MCPParams) (any, error) {
	fn, ok := mcpTools[p.Tool]
	if !ok {
		return nil, badRequest("unknown tool %q", p.Tool)
	}
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return protocol.MCPResult{Text: "this agent is not part of a known Multiplex session", IsError: true}, nil
	}
	text, err := fn(d, ctx, s, p.Args)
	if err != nil {
		msg := err.Error()
		if pe := toProtocolError(err); pe.Code != protocol.CodeInternal {
			msg = pe.Message
		}
		return protocol.MCPResult{Text: msg, IsError: true}, nil
	}
	return protocol.MCPResult{Text: text}, nil
}

// decodeArgs parses a tool's JSON arguments into v. Empty arguments are {}.
func decodeArgs(raw json.RawMessage, v any) error {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return badRequest("invalid arguments: %v", err)
	}
	return nil
}

// Limits on what an agent can store through MCP tools.
const (
	maxTitle   = 200
	maxBody    = 8000
	maxItem    = 1000
	maxItems   = 100
	maxEvidece = 500
)

// clipText cuts s to at most n bytes at a rune boundary.
func clipText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0x80 {
		cut = cut[:len(cut)-1]
	}
	if len(cut) > 0 && cut[len(cut)-1] >= 0xC0 {
		cut = cut[:len(cut)-1]
	}
	return cut + "…"
}

// clipList trims, clips and caps a list of strings, dropping empty items.
func clipList(in []string, n int) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = clipText(s, n); s != "" {
			out = append(out, s)
		}
		if len(out) == maxItems {
			break
		}
	}
	return out
}

// toJSONText renders v as indented JSON for a tool result.
func toJSONText(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode result: %w", err)
	}
	return string(b), nil
}
