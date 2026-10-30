// Package core holds Multiplex's domain types: workspaces, sessions, panes,
// summaries, insight notes, review comments, checkpoints and approvals.
//
// core has no dependencies outside the standard library. Every other
// package speaks in these types, so replacing an implementation (a store,
// an agent adapter, a VCS backend) never changes the shape of the data.
package core
