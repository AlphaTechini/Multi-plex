// Package broker is the daemon-side coordinator for human-present approvals
// (docs/04-safety.md Layer 5). Gate code calls Request and blocks; a UI client
// is told about the pending request through OnPending and answers with
// Respond. Denying needs no verification; approving runs the best available
// approval.Verifier (Windows Hello, polkit, ...) or, when the machine has none,
// the typed one-time code shown only in the TUI modal.
package broker
