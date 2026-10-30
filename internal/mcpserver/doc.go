// Package mcpserver is `mpx mcp`: a stdio MCP server that gives an agent the
// Multiplex tools (report_progress, note_to_user, the spec tools and the
// orchestration tools). It holds no logic of its own: every tool call is
// forwarded to the daemon as an MCP request carrying the session ID from
// MPX_SESSION_ID, and the daemon's text answer is returned.
//
// stdout is the protocol channel, so nothing in this package prints to it.
// There is deliberately no tool to approve a gated action (docs/10).
package mcpserver
