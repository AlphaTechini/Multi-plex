package term

import (
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/vt"
)

// dirtyReporter is implemented by emulators that can say which rows were
// touched since the last call (charmvt does). It lets the framer render only
// those rows instead of every row on every frame.
type dirtyReporter interface {
	TakeDirty(dst []int) []int
}

// fullCheckEvery is how often (in frames) a reporting emulator is also
// compared row by row, as a guard against a missed damage mark.
const fullCheckEvery = 64

// framer turns emulator state into PaneFrames. It remembers the rows it last
// sent and reports only rows whose rendering differs, so a slow consumer
// loses intermediate frames but never a change. It is not safe for
// concurrent use; the Host serialises access.
type framer struct {
	id  core.PaneID
	emu vt.Emulator
	rep dirtyReporter // nil if the emulator cannot report damage

	cache      []string // last sent text of every row
	cols, rows int      // size of the cache
	full       bool     // the next frame must carry every row
	sentCursor vt.Cursor
	sentTitle  string
	sentAlt    bool
	frames     int
	dirty      []int // scratch
}

func newFramer(id core.PaneID, emu vt.Emulator) *framer {
	f := &framer{id: id, emu: emu, full: true}
	f.rep, _ = emu.(dirtyReporter)
	return f
}

// forceFull makes the next frame carry every row (after a resize, or when a
// client has lost sync).
func (f *framer) forceFull() { f.full = true }

// next computes the next frame. ok is false when nothing changed since the
// previous frame, so there is nothing to send.
func (f *framer) next() (fr protocol.PaneFrame, ok bool) {
	cols, rows := f.emu.Size()
	alt := f.emu.AltScreen()
	if cols != f.cols || rows != f.rows {
		f.cache = make([]string, rows)
		f.cols, f.rows = cols, rows
		f.full = true
	}
	if alt != f.sentAlt {
		f.full = true // the whole screen was swapped
	}
	f.frames++

	// Take the damage marks before reading rows: a write that lands in
	// between marks its row again and is picked up by the next frame.
	f.dirty = f.dirty[:0]
	checkAll := f.full || f.rep == nil || f.frames%fullCheckEvery == 0
	if f.rep != nil {
		f.dirty = f.rep.TakeDirty(f.dirty)
	}

	var changed []protocol.Row
	if checkAll {
		changed = make([]protocol.Row, 0, rows)
		for y := 0; y < rows; y++ {
			changed = f.collect(changed, y)
		}
	} else if len(f.dirty) > 0 {
		changed = make([]protocol.Row, 0, len(f.dirty))
		for _, y := range f.dirty {
			if y >= 0 && y < rows {
				changed = f.collect(changed, y)
			}
		}
	}

	cur := f.emu.Cursor()
	title := f.emu.Title()
	if !f.full && len(changed) == 0 && cur == f.sentCursor && title == f.sentTitle {
		return protocol.PaneFrame{}, false
	}
	fr = protocol.PaneFrame{
		PaneID:        f.id,
		Cols:          cols,
		Rows:          rows,
		Full:          f.full,
		Changed:       changed,
		CursorX:       cur.X,
		CursorY:       cur.Y,
		CursorVisible: cur.Visible,
		Title:         title,
	}
	f.full = false
	f.sentCursor, f.sentTitle, f.sentAlt = cur, title, alt
	return fr, true
}

// collect renders row y and appends it to rows if it differs from what was
// last sent (always, for a full frame), updating the cache.
func (f *framer) collect(rows []protocol.Row, y int) []protocol.Row {
	line := f.emu.Line(y)
	if f.full || line != f.cache[y] {
		f.cache[y] = line
		rows = append(rows, protocol.Row{Y: y, Text: line})
	}
	return rows
}

// snapshot renders a full frame without touching the framer's state.
func snapshot(id core.PaneID, emu vt.Emulator) protocol.PaneFrame {
	cols, rows := emu.Size()
	changed := make([]protocol.Row, rows)
	for y := range changed {
		changed[y] = protocol.Row{Y: y, Text: emu.Line(y)}
	}
	cur := emu.Cursor()
	return protocol.PaneFrame{
		PaneID:        id,
		Cols:          cols,
		Rows:          rows,
		Full:          true,
		Changed:       changed,
		CursorX:       cur.X,
		CursorY:       cur.Y,
		CursorVisible: cur.Visible,
		Title:         emu.Title(),
	}
}
