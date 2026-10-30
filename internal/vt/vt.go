// Package vt is the neutral layer for terminal emulation. A pane's PTY
// output is written into an Emulator, which keeps the screen state; clients
// render from it. The implementation (pure Go today, possibly libghostty-vt
// later, see docs/02-tech-stack.md Q1) lives in a subpackage.
package vt

import "io"

// Cursor is the emulator's cursor state.
type Cursor struct {
	X, Y    int
	Visible bool
}

// Emulator keeps the state of one terminal screen.
//
// Implementations must be safe for concurrent use: the daemon writes PTY
// output from one goroutine and reads screen lines from another.
type Emulator interface {
	// Write feeds raw PTY output into the emulator.
	io.Writer
	// Resize changes the screen size.
	Resize(cols, rows int)
	// Size returns the current screen size.
	Size() (cols, rows int)
	// Line returns screen row y rendered as a string with ANSI SGR styling,
	// exactly cols cells wide (pad with spaces). Out of range returns "".
	Line(y int) string
	// Cursor returns the cursor position and visibility.
	Cursor() Cursor
	// AltScreen reports whether the alternate screen is active.
	AltScreen() bool
	// Title returns the window title the program set, if any.
	Title() string
	// SetReplyWriter sets where the emulator writes replies to queries
	// (device attributes, cursor position reports). The daemon passes the
	// PTY so programs that query the terminal do not hang.
	SetReplyWriter(w io.Writer)
	// Close releases resources.
	Close() error
}

// Factory creates an emulator of the given size.
type Factory func(cols, rows int) Emulator
