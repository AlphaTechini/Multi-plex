// Package charmvt implements the vt package's Emulator with the pure-Go
// github.com/charmbracelet/x/vt, which keeps the build free of cgo
// (docs/02-tech-stack.md Q1).
package charmvt

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	xvt "github.com/charmbracelet/x/vt"

	"github.com/alphatechini/multi-plex/internal/vt"
)

// Factory is the vt.Factory for this implementation.
var Factory vt.Factory = New

// emulator wraps x/vt's Emulator, which is not safe for concurrent use,
// behind one mutex. The mutex is held for a whole Write, Resize or Line.
type emulator struct {
	mu         sync.Mutex
	e          *xvt.Emulator
	cols, rows int
	title      string
	closed     bool
	blank      string // cached all-spaces row of width cols

	pw      *io.PipeWriter // the emulator's reply pipe, closed to stop the pump
	replies *replyPump

	line []byte // scratch buffer reused by Line
}

var _ vt.Emulator = (*emulator)(nil)

// New creates an emulator of the given size (each dimension at least 1).
func New(cols, rows int) vt.Emulator {
	cols, rows = max(cols, 1), max(rows, 1)
	em := &emulator{e: xvt.NewEmulator(cols, rows), cols: cols, rows: rows}
	em.blank = strings.Repeat(" ", cols)
	em.e.SetCallbacks(xvt.Callbacks{
		// Called from inside Write, with mu already held.
		Title: func(s string) { em.title = s },
	})
	// x/vt does not answer the Kitty keyboard query; answer "no flags" so
	// programs do not wait for a reply that never comes.
	em.e.RegisterCsiHandler(ansi.Command('?', 0, 'u'), func(ansi.Params) bool {
		_, _ = io.WriteString(em.e.InputPipe(), "\x1b[?0u")
		return true
	})
	em.pw, _ = em.e.InputPipe().(*io.PipeWriter)
	em.replies = startReplyPump(em.e)
	return em
}

// Write feeds PTY output into the emulator. A panic inside the library is
// turned into an error so one malformed stream cannot take the daemon down.
func (em *emulator) Write(p []byte) (n int, err error) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.closed {
		return 0, io.ErrClosedPipe
	}
	defer func() {
		if r := recover(); r != nil {
			n, err = 0, fmt.Errorf("charmvt: emulator panic: %v", r)
			em.repair()
		}
	}()
	return em.e.Write(p)
}

// repair gets the emulator out of the state a library panic left it in: CAN
// abandons a half-parsed sequence, and resetting the scroll region removes
// the usual culprit (a region taller than the screen), which would otherwise
// make every following scroll panic again.
func (em *emulator) repair() {
	defer func() { _ = recover() }()
	_, _ = em.e.Write([]byte("\x18\x1b[r"))
}

// Resize changes the screen size (each dimension at least 1).
func (em *emulator) Resize(cols, rows int) {
	cols, rows = max(cols, 1), max(rows, 1)
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.closed || (cols == em.cols && rows == em.rows) {
		return
	}
	defer func() { _ = recover() }()
	em.e.Resize(cols, rows)
	if cols != em.cols {
		em.blank = strings.Repeat(" ", cols)
	}
	em.cols, em.rows = cols, rows
}

// Size returns the current screen size.
func (em *emulator) Size() (cols, rows int) {
	em.mu.Lock()
	defer em.mu.Unlock()
	return em.cols, em.rows
}

// Cursor returns the cursor position and visibility.
func (em *emulator) Cursor() vt.Cursor {
	em.mu.Lock()
	defer em.mu.Unlock()
	p := em.e.CursorPosition()
	return vt.Cursor{X: p.X, Y: p.Y, Visible: !em.e.CursorHidden()}
}

// AltScreen reports whether the alternate screen is active.
func (em *emulator) AltScreen() bool {
	em.mu.Lock()
	defer em.mu.Unlock()
	return em.e.IsAltScreen()
}

// Title returns the window title set with OSC 0 or 2.
func (em *emulator) Title() string {
	em.mu.Lock()
	defer em.mu.Unlock()
	return em.title
}

// TakeDirty appends to dst the indexes of the rows touched since the last
// call and clears the marks (x/vt never clears them itself). The marks follow
// the active screen, so after an alternate screen switch the caller must
// compare every row (see AltScreen). Hosts use it through an optional
// interface to avoid re-rendering untouched rows.
func (em *emulator) TakeDirty(dst []int) []int {
	em.mu.Lock()
	defer em.mu.Unlock()
	t := em.e.Touched()
	for y, ld := range t {
		if ld != nil {
			dst = append(dst, y)
		}
	}
	clear(t)
	return dst
}

// SetReplyWriter sets where replies to terminal queries go. Replies are
// queued and written by a separate goroutine, so a slow or stuck writer never
// blocks Write. A nil writer discards replies.
func (em *emulator) SetReplyWriter(w io.Writer) { em.replies.setWriter(w) }

// Close stops the reply pump and releases the emulator. It is idempotent.
func (em *emulator) Close() error {
	em.mu.Lock()
	if em.closed {
		em.mu.Unlock()
		return nil
	}
	em.closed = true
	em.mu.Unlock()

	// Closing the pipe from our side ends the drain goroutine without
	// touching x/vt's unsynchronised "closed" flag, which its Read checks.
	var err error
	if em.pw != nil {
		_ = em.pw.CloseWithError(io.EOF)
		em.replies.stop()
		em.mu.Lock()
		err = em.e.Close()
		em.mu.Unlock()
	} else {
		err = errors.New("charmvt: reply pipe unavailable")
	}
	return err
}
