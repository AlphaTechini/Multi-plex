// Package safety is the parent of Multiplex's enforcement layers
// (docs/04-safety.md). The layers live in subpackages so each one can be
// replaced or tested alone:
//
//   - pathpolicy: Layers 2 and 3 for file tools (writes outside the worktree,
//     credential stores on the deny-read list, protected settings files).
//   - envscrub:   Layer 3 environment scrubbing for agent processes.
//   - classify:   Layer 4 command classification, driven by rules that are data.
//   - gate:       combines the layers for one pre-tool hook event and builds
//     the approval request that Layer 5 shows to the human.
//
// Nothing here weakens a default: user rule files can only add rules.
package safety
