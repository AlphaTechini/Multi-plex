// Package pty is the neutral layer for pseudo-terminals. The daemon only
// talks to the Spawner and Process interfaces; the implementation (today
// xpty: Unix PTY and Windows ConPTY) lives in a subpackage and can be
// swapped without touching callers.
package pty

import (
	"context"
	"io"
)

// Spec describes a process to start inside a new pseudo-terminal.
type Spec struct {
	Argv []string // program and arguments; Argv[0] is resolved via PATH
	Dir  string   // working directory
	Env  []string // full environment ("KEY=value"); nil means inherit
	Cols int
	Rows int
}

// Process is a running program attached to a pseudo-terminal.
// Read returns the terminal's output, Write sends input to it.
type Process interface {
	io.Reader
	io.Writer
	// Resize changes the terminal size.
	Resize(cols, rows int) error
	// Wait blocks until the process exits and returns its exit code.
	// It must work on Windows ConPTY, where exec.Cmd.Wait cannot be used.
	Wait() (exitCode int, err error)
	// Kill stops the process.
	Kill() error
	// Close releases the terminal. Safe to call more than once.
	Close() error
	// Pid returns the process id.
	Pid() int
}

// Spawner starts processes in pseudo-terminals.
type Spawner interface {
	Spawn(ctx context.Context, spec Spec) (Process, error)
}
