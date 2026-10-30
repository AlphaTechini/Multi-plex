package diffview

import (
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// dlKind is the kind of a display line of the diff body.
type dlKind uint8

// Display line kinds.
const (
	dlUnit  dlKind = iota // a diff row (unit)
	dlBox                 // a line of an inline comment box
	dlBlank               // blank row before the end marker
	dlEOF                 // end-of-file marker
)

// dline is one display line of the diff body. Scrolling moves over display
// lines; the cursor moves over units.
type dline struct {
	kind dlKind
	unit int32 // dlUnit: the unit; dlBox: the unit the box hangs under (-1: after the last unit)
	box  int32 // dlBox: index into fileView.boxes
	sub  int32 // dlBox: line of the box
}

// box is the rendered lines of one inline comment box.
type box struct {
	lines []string
	// placed is the index of the comment in fileView.placed, or -1 for the
	// composer.
	placed int
}

// rcEntry is a cached rendered unit.
type rcEntry struct {
	s   string
	gen uint32
}

// fileView is the state of the file currently shown: its rows, units,
// display lines, comment boxes, cursor, selection and scroll position.
type fileView struct {
	e   *fileEntry
	tok *fileTokens

	rows  []diff.Row
	units []unit
	nw    int // digits per line-number column
	sbs   bool

	placed []placedComment
	after  map[int][]int // unit → boxes under it
	tail   []int         // boxes before the end marker
	boxes  []box

	dl   []dline
	dlOf []int32 // unit → its display line

	// w is the width the boxes were laid out for.
	w int

	cursor, sel int // unit indexes; sel is -1 without a selection
	top         int // first visible display line

	rc  []rcEntry
	gen uint32 // bumped to invalidate every cached row
}

// fileNote returns the file's display note: the empty-state text shown instead
// of rows for binary, mode-only, deleted-without-diff, huge and similar files.
// It returns "" for files that show rows.
func (m *Model) fileNote(e *fileEntry) (title, body string) {
	f := &e.file
	switch {
	case f.Binary:
		return "Binary file", "Binary files are not shown. The file " + verb(e) + "."
	case e.huge && !m.forceHuge[e.path]:
		return "Very large diff", "This file has more than 40,000 changed lines. Press enter to open it anyway."
	case len(f.Hunks) > 0:
		return "", ""
	case f.ModeOnly():
		return "Mode changed", f.OldMode + " " + m.t.Glyphs.ArrowRight + " " + f.NewMode + ". The contents did not change."
	case f.Status == vcs.Deleted:
		return "File deleted", "The file was removed. No lines are shown for an empty file."
	case f.Status == vcs.Renamed:
		return "File renamed", "Renamed from " + f.OldPath + " with no changes to the contents."
	case f.Status == vcs.Added:
		return "Empty file added", "The new file has no content."
	}
	return "No line changes", "Git reports this file as changed but there are no lines to show."
}

func verb(e *fileEntry) string {
	switch e.file.Status {
	case vcs.Added:
		return "was added"
	case vcs.Deleted:
		return "was deleted"
	}
	return "changed"
}

// hasRows reports whether the file shows diff rows (and so a cursor).
func (m *Model) hasRows(e *fileEntry) bool {
	t, _ := m.fileNote(e)
	return t == "" && len(e.file.Hunks) > 0
}

// bodyH is the number of rows of the diff body.
func (m *Model) bodyH() int { return max(m.h-bodyTop, 1) }

// maxTop is the largest scroll offset: the end marker may reach the last
// row, and the view never scrolls into another file.
func (m *Model) maxTop() int {
	if m.fv == nil {
		return 0
	}
	return max(len(m.fv.dl)-m.bodyH(), 0)
}

// clampTop keeps the scroll offset inside the file.
func (m *Model) clampTop() {
	if m.fv != nil {
		m.fv.top = min(max(m.fv.top, 0), m.maxTop())
	}
}

// visibleUnits returns the first and last unit that are on screen.
func (m *Model) visibleUnits() (lo, hi int) {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return 0, 0
	}
	at := func(i int) int {
		i = min(max(i, 0), len(fv.dl)-1)
		for ; i >= 0; i-- {
			if d := fv.dl[i]; d.kind == dlUnit || d.kind == dlBox && d.unit >= 0 {
				return int(d.unit)
			}
		}
		return 0
	}
	return at(fv.top), at(fv.top + m.bodyH())
}
