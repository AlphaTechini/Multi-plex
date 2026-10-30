package daemon

import (
	"context"
	"encoding/json"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// handler runs one request. The raw params are decoded by typed.
type handler func(ctx context.Context, c *conn, raw json.RawMessage) (any, error)

// route is a method's handler plus how the connection runs it.
type route struct {
	h      handler
	inline bool // run on the read loop, in order (must be quick)
}

// typed decodes the request's params into P and calls fn.
func typed[P any](fn func(ctx context.Context, c *conn, p P) (any, error)) handler {
	return func(ctx context.Context, c *conn, raw json.RawMessage) (any, error) {
		var p P
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, badRequest("invalid params: %v", err)
			}
		}
		return fn(ctx, c, p)
	}
}

// buildRoutes lists every protocol method and its handler.
func (d *Daemon) buildRoutes() map[string]route {
	r := map[string]route{}
	add := func(method string, h handler) { r[method] = route{h: h} }
	inline := func(method string, h handler) { r[method] = route{h: h, inline: true} }

	inline(protocol.MethodHello, typed(d.hHello))
	inline(protocol.MethodSubscribe, typed(d.hSubscribe))
	add(protocol.MethodShutdown, typed(d.hShutdown))

	add(protocol.MethodWorkspaceOpen, typed(d.hWorkspaceOpen))
	add(protocol.MethodWorkspaceList, typed(d.hWorkspaceList))

	add(protocol.MethodSessionCreate, typed(d.hSessionCreate))
	add(protocol.MethodSessionList, typed(d.hSessionList))
	add(protocol.MethodSessionGet, typed(d.hSessionGet))
	add(protocol.MethodSessionStop, typed(d.hSessionStop))
	add(protocol.MethodSessionRestart, typed(d.hSessionRestart))
	add(protocol.MethodSessionRemove, typed(d.hSessionRemove))
	add(protocol.MethodSessionSeen, typed(d.hSessionSeen))
	add(protocol.MethodSessionFollow, typed(d.hSessionFollow))
	add(protocol.MethodSessionMessage, typed(d.hSessionMessage))
	add(protocol.MethodSessionInterrupt, typed(d.hSessionInterrupt))
	add(protocol.MethodSessionPush, typed(d.hSessionPush))

	add(protocol.MethodPaneOpen, typed(d.hPaneOpen))
	add(protocol.MethodPaneClose, typed(d.hPaneClose))
	inline(protocol.MethodPaneInput, typed(d.hPaneInput))
	inline(protocol.MethodPaneResize, typed(d.hPaneResize))
	add(protocol.MethodPaneSnapshot, typed(d.hPaneSnapshot))

	add(protocol.MethodDiffGet, typed(d.hDiffGet))
	add(protocol.MethodFileRead, typed(d.hFileRead))

	add(protocol.MethodCommentPut, typed(d.hCommentPut))
	add(protocol.MethodCommentList, typed(d.hCommentList))
	add(protocol.MethodCommentDelete, typed(d.hCommentDelete))
	add(protocol.MethodCommentSend, typed(d.hCommentSend))

	add(protocol.MethodNoteList, typed(d.hNoteList))
	add(protocol.MethodNoteAct, typed(d.hNoteAct))
	add(protocol.MethodSummaryGet, typed(d.hSummaryGet))

	add(protocol.MethodApprovalList, typed(d.hApprovalList))
	add(protocol.MethodApprovalRespond, typed(d.hApprovalRespond))
	add(protocol.MethodApprovalAudit, typed(d.hApprovalAudit))

	add(protocol.MethodCheckpointList, typed(d.hCheckpointList))
	add(protocol.MethodCheckpointRestore, typed(d.hCheckpointRestore))

	add(protocol.MethodAgentList, typed(d.hAgentList))
	add(protocol.MethodRoleList, typed(d.hRoleList))

	add(protocol.MethodSpecList, typed(d.hSpecList))
	add(protocol.MethodSpecGet, typed(d.hSpecGet))

	add(protocol.MethodHook, typed(d.hHook))
	add(protocol.MethodMCP, typed(d.hMCP))
	return r
}
