package diffview

import (
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// rebuild recomputes the rows, units, comment placement and display lines of
// the current file from its model and content. The caller restores the
// cursor and scroll position (capturePos/restorePos) when they matter.
func (m *Model) rebuild() {
	fv := m.fv
	if fv == nil {
		return
	}
	e := fv.e
	var opts diff.RowOptions
	total := 0
	if c := m.contents[e.path]; c != nil && !c.binary && e.file.Status != vcs.Deleted {
		total = len(c.lines)
		opts.TotalNewLines = total
	}
	if m.hasRows(e) {
		fv.rows = diff.Rows(&e.file, opts)
		fv.units = buildUnits(&e.file, fv.rows, fv.sbs)
	} else {
		fv.rows, fv.units = nil, nil
	}
	fv.nw = numberWidth(&e.file, total)
	m.placeComments()
	m.layout()
}

// layout builds the comment boxes and the display lines from the units and
// placed comments. It is cheap enough to run on every keystroke of a comment
// being typed.
func (m *Model) layout() {
	fv := m.fv
	if fv == nil {
		return
	}
	m.buildBoxes()
	n := len(fv.units)
	dl := fv.dl[:0]
	if cap(dl) < n+len(fv.boxes)*4+2 {
		dl = make([]dline, 0, n+len(fv.boxes)*4+2)
	}
	if cap(fv.dlOf) < n {
		fv.dlOf = make([]int32, n)
	}
	fv.dlOf = fv.dlOf[:n]
	for i := range n {
		fv.dlOf[i] = int32(len(dl))
		dl = append(dl, dline{kind: dlUnit, unit: int32(i)})
		for _, bi := range fv.after[i] {
			for s := range fv.boxes[bi].lines {
				dl = append(dl, dline{kind: dlBox, unit: int32(i), box: int32(bi), sub: int32(s)})
			}
		}
	}
	for _, bi := range fv.tail {
		for s := range fv.boxes[bi].lines {
			dl = append(dl, dline{kind: dlBox, unit: -1, box: int32(bi), sub: int32(s)})
		}
	}
	if n > 0 {
		dl = append(dl, dline{kind: dlBlank}, dline{kind: dlEOF})
	}
	fv.dl = dl
	fv.gen++
	if len(fv.rc) < n {
		fv.rc = make([]rcEntry, n)
	}
	fv.cursor = min(max(fv.cursor, 0), max(n-1, 0))
	if fv.sel >= n {
		fv.sel = -1
	}
	m.clampTop()
}

// posState is a position in a file that survives a rebuild: cursor,
// selection and scroll are remembered by line identity, not by index.
type posState struct {
	cursor, sel, top lineID
	cursorKind       unitKind
	topKind          unitKind
	topSub           int
	hasSel           bool
	cmp              lineID
}

// capturePos remembers where the view is.
func (m *Model) capturePos() posState {
	fv := m.fv
	var p posState
	if fv == nil || len(fv.units) == 0 {
		return p
	}
	f := &fv.e.file
	p.cursor = fv.units[fv.cursor].id(f)
	p.cursorKind = fv.units[fv.cursor].kind
	if fv.sel >= 0 {
		p.hasSel, p.sel = true, fv.units[fv.sel].id(f)
	}
	if fv.top < len(fv.dl) {
		d := fv.dl[min(fv.top, len(fv.dl)-1)]
		ui := int(d.unit)
		if d.kind == dlBlank || d.kind == dlEOF || ui < 0 {
			ui = len(fv.units) - 1
		}
		p.top = fv.units[ui].id(f)
		p.topKind = fv.units[ui].kind
		p.topSub = fv.top - int(fv.dlOf[ui])
	}
	if m.cmp != nil && m.cmp.unit >= 0 && m.cmp.unit < len(fv.units) {
		p.cmp = fv.units[m.cmp.unit].id(f)
	}
	return p
}

// restorePos puts the view back after a rebuild.
func (m *Model) restorePos(p posState) {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return
	}
	fv.cursor = m.findUnitKind(p.cursor, p.cursorKind)
	fv.sel = -1
	if p.hasSel {
		fv.sel = m.findUnit(p.sel)
	}
	if m.cmp != nil {
		m.cmp.unit = m.findUnit(p.cmp)
		m.layout()
	}
	ui := m.findUnitKind(p.top, p.topKind)
	fv.top = int(fv.dlOf[ui]) + p.topSub
	m.clampTop()
}

// findUnitKind is findUnit that lands on the hunk header or fold row the
// position was on, not on the first line after it.
func (m *Model) findUnitKind(id lineID, kind unitKind) int {
	fv := m.fv
	ui := m.findUnit(id)
	switch kind {
	case uHunk:
		if ui > 0 && fv.units[ui-1].kind == uHunk {
			return ui - 1
		}
	case uFold:
		if id == (lineID{}) {
			for i := len(fv.units) - 1; i >= 0; i-- {
				if fv.units[i].kind == uFold {
					return i
				}
			}
			return ui
		}
		if ui > 0 && fv.units[ui-1].kind == uFold {
			return ui - 1
		}
		if ui > 1 && fv.units[ui-1].kind == uHunk && fv.units[ui-2].kind == uFold {
			return ui - 2
		}
	}
	return ui
}

// findUnit returns the unit holding the line with the given identity, or the
// unit with the nearest earlier line.
func (m *Model) findUnit(id lineID) int {
	fv := m.fv
	f := &fv.e.file
	best, bestNo := 0, int32(-1)
	for i, u := range fv.units {
		if u.kind == uHunk || u.kind == uFold {
			continue
		}
		l, r := u.lines(f)
		for _, ln := range [2]*diff.Line{l, r} {
			if ln == nil {
				continue
			}
			if idOf(ln) == id {
				return i
			}
			no := int32(ln.NewNo)
			want := id.new
			if id.new == 0 {
				no, want = int32(ln.OldNo), id.old
			}
			if no > 0 && no <= want && no > bestNo {
				best, bestNo = i, no
			}
		}
	}
	return best
}

// findUnitByNew returns the unit holding new-side line no, or the nearest
// following one.
func (m *Model) findUnitByNew(no int) int {
	fv := m.fv
	for i, u := range fv.units {
		if _, r := u.lines(&fv.e.file); r != nil && r.NewNo >= no {
			return i
		}
	}
	return max(len(fv.units)-1, 0)
}
