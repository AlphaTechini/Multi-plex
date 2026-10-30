// Package daemon is the Multiplex daemon: the session manager, pane host,
// event bus, hook gate and MCP back end behind `mpx daemon`
// (docs/01-architecture.md).
//
// Every dependency is an interface from a neutral package and is injected
// through Deps, so tests run the whole daemon in-process against an in-memory
// store. The concrete implementations are chosen in one place, cmd/mpx.
//
// Layout, one major function or type per file:
//
//	daemon.go, serve.go      the Daemon type, Serve and shutdown
//	server.go, route.go      connection handling and method dispatch
//	bus.go                   the event bus with per-subscriber queues
//	session_*.go             session operations
//	pane_*.go                pane operations
//	hook*.go                 the hook gate (docs/04-safety.md)
//	mcp_*.go                 MCP tool back ends (docs/08, 09, 10)
//	autostart*.go            EnsureDaemon, used by the TUI
package daemon
