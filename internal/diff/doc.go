// Package diff is the pure-Go unified-diff engine behind the diff viewer
// (docs/07-diff-viewer.md). It parses `git diff` output into a model built for
// fast, virtualised rendering, adds word-level highlights, produces the row
// list for one file (with collapsed unchanged regions), the side-by-side
// pairing, comment anchoring, and the message sent to the agent.
//
// The package does no I/O and depends only on the standard library and the
// neutral vcs package for the file status type.
package diff
