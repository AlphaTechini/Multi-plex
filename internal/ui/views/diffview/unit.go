package diffview

import "github.com/alphatechini/multi-plex/internal/diff"

// unitKind says what a selectable unit of the diff body is.
type unitKind uint8

// Unit kinds.
const (
	// uHunk is a hunk header row.
	uHunk unitKind = iota
	// uLine is one line of the unified view.
	uLine
	// uFold is a collapsed run of unchanged lines.
	uFold
	// uPair is one row of the side-by-side view (a left and/or right line).
	uPair
)

// unit is one selectable display row of the diff body: the cursor, the
// selection and comment anchors all move in units. In the unified view a
// unit is a diff row; in side-by-side it is a pair of lines.
type unit struct {
	kind unitKind
	// hunk indexes File.Hunks (uHunk, uLine, uPair); for uFold it is the
	// hunk that follows the gap.
	hunk int32
	// row indexes the diff.Row the unit came from.
	row int32
	// l and r index Hunk.Lines: the old-side and new-side line (-1 when
	// absent). In the unified view both hold the same index.
	l, r int32
}

// lineID identifies a diff line by its numbers on each side (0 = absent).
type lineID struct{ old, new int32 }

// lines returns the old-side and new-side line of a unit (nil when absent).
func (u unit) lines(f *diff.File) (left, right *diff.Line) {
	if u.kind != uLine && u.kind != uPair {
		return nil, nil
	}
	h := &f.Hunks[u.hunk]
	if u.l >= 0 {
		left = &h.Lines[u.l]
	}
	if u.r >= 0 {
		right = &h.Lines[u.r]
	}
	if u.kind == uLine && left != nil {
		// A single unified line sits on the side(s) it exists on.
		switch left.Kind {
		case diff.Add:
			left = nil
		case diff.Del:
			right = nil
		}
	}
	return left, right
}

// isChange reports whether the unit holds an added or deleted line.
func (u unit) isChange(f *diff.File) bool {
	l, r := u.lines(f)
	return (l != nil && l.Kind == diff.Del) || (r != nil && r.Kind == diff.Add)
}

// id returns the unit's line identity (zero for headers and folds).
func (u unit) id(f *diff.File) lineID {
	switch u.kind {
	case uHunk:
		h := &f.Hunks[u.hunk]
		if len(h.Lines) > 0 {
			return idOf(&h.Lines[0])
		}
		return lineID{}
	case uFold:
		if u.l >= 0 && int(u.hunk) < len(f.Hunks) && int(u.l) < len(f.Hunks[u.hunk].Lines) {
			return idOf(&f.Hunks[u.hunk].Lines[u.l])
		}
		return lineID{}
	}
	l, r := u.lines(f)
	switch {
	case r != nil:
		return idOf(r)
	case l != nil:
		return idOf(l)
	}
	return lineID{}
}

// idOf returns the identity of a line.
func idOf(l *diff.Line) lineID { return lineID{int32(l.OldNo), int32(l.NewNo)} }

// buildUnits turns diff rows into units.
func buildUnits(f *diff.File, rows []diff.Row, sbs bool) []unit {
	units := make([]unit, 0, len(rows))
	if !sbs {
		for i, r := range rows {
			switch r.Kind {
			case diff.RowHunkHeader:
				units = append(units, unit{kind: uHunk, hunk: int32(r.Hunk), row: int32(i), l: -1, r: -1})
			case diff.RowLine:
				units = append(units, unit{kind: uLine, hunk: int32(r.Hunk), row: int32(i), l: int32(r.Line), r: int32(r.Line)})
			case diff.RowCollapsed:
				units = append(units, foldUnit(f, r, i))
			}
		}
		return units
	}
	var (
		pairs   []diff.Pair
		pairOf  []int
		emitted []bool
	)
	for i, r := range rows {
		switch r.Kind {
		case diff.RowHunkHeader:
			h := f.Hunks[r.Hunk]
			pairs = diff.SideBySide(h)
			pairOf = make([]int, len(h.Lines))
			emitted = make([]bool, len(pairs))
			for pi, p := range pairs {
				if p.Left >= 0 {
					pairOf[p.Left] = pi
				}
				if p.Right >= 0 {
					pairOf[p.Right] = pi
				}
			}
			units = append(units, unit{kind: uHunk, hunk: int32(r.Hunk), row: int32(i), l: -1, r: -1})
		case diff.RowLine:
			pi := pairOf[r.Line]
			if emitted[pi] {
				continue
			}
			emitted[pi] = true
			p := pairs[pi]
			units = append(units, unit{kind: uPair, hunk: int32(r.Hunk), row: int32(i), l: int32(p.Left), r: int32(p.Right)})
		case diff.RowCollapsed:
			units = append(units, foldUnit(f, r, i))
		}
	}
	return units
}

// foldUnit builds the unit of a collapsed row. Its l field remembers the
// first line after the fold (for positions that survive a rebuild).
func foldUnit(f *diff.File, r diff.Row, row int) unit {
	u := unit{kind: uFold, hunk: int32(r.Hunk), row: int32(row), l: -1, r: -1}
	if r.Hunk < len(f.Hunks) {
		after := r.Line + r.Collapsed // inside a hunk
		if r.Line == 0 {
			after = 0 // gap before a hunk
		}
		if after < len(f.Hunks[r.Hunk].Lines) {
			u.l = int32(after)
		}
	}
	return u
}

// numberWidth returns the digits needed for the file's line numbers (at
// least 4, as in the design).
func numberWidth(f *diff.File, total int) int {
	maxNo := total
	for i := range f.Hunks {
		h := &f.Hunks[i]
		maxNo = max(maxNo, h.OldStart+h.OldLines, h.NewStart+h.NewLines)
	}
	n := 4
	for v := maxNo; v >= 10000; v /= 10 {
		n++
	}
	return n
}
