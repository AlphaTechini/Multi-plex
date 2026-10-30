package diffview

// keepCursorVisible scrolls so the cursor (and the comment box under it
// while one is being written) is on screen, with a few lines of margin.
func (m *Model) keepCursorVisible() {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return
	}
	h := m.bodyH()
	off := min(scrolloff, (h-1)/2)
	first := int(fv.dlOf[fv.cursor])
	last := first
	if m.cmp != nil && m.cmp.unit >= 0 && m.cmp.unit < len(fv.units) {
		cu := m.cmp.unit
		first = min(first, int(fv.dlOf[cu]))
		end := int(fv.dlOf[cu])
		for _, bi := range fv.after[cu] {
			end += len(fv.boxes[bi].lines)
		}
		last = max(last, end)
	}
	if first-off < fv.top {
		fv.top = first - off
	}
	if last+off >= fv.top+h {
		fv.top = last + off - h + 1
	}
	m.clampTop()
}

// moveCursor moves the cursor by delta units, extending the selection when
// extend is set. The cursor stays on the unit at either end.
func (m *Model) moveCursor(delta int, extend bool) {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return
	}
	m.noteScroll()
	if extend && fv.sel < 0 {
		fv.sel = fv.cursor
	}
	if !extend && !m.visual {
		fv.sel = -1
	}
	fv.cursor = min(max(fv.cursor+delta, 0), len(fv.units)-1)
	m.keepCursorVisible()
}

// scrollBy scrolls the view by n display lines without moving the cursor.
func (m *Model) scrollBy(n int) {
	if m.fv == nil {
		return
	}
	m.noteScroll()
	m.fv.top += n
	m.clampTop()
}

// halfPage moves the cursor and the view by half a screen.
func (m *Model) halfPage(dir int) {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return
	}
	m.noteScroll()
	step := max(m.bodyH()/2, 1)
	// Move the cursor by the units that fit in half a screen.
	target := int(fv.dlOf[fv.cursor]) + dir*step
	ui := fv.cursor
	for ui+dir >= 0 && ui+dir < len(fv.units) && (int(fv.dlOf[ui+dir])-target)*dir <= 0 {
		ui += dir
	}
	if ui == fv.cursor {
		ui = min(max(ui+dir, 0), len(fv.units)-1)
	}
	if !m.visual {
		fv.sel = -1
	}
	fv.cursor = ui
	fv.top += dir * step
	m.clampTop()
	m.keepCursorVisible()
}

// goTop moves to the first unit.
func (m *Model) goTop() {
	if m.fv == nil || len(m.fv.units) == 0 {
		return
	}
	m.noteScroll()
	if !m.visual {
		m.fv.sel = -1
	}
	m.fv.cursor, m.fv.top = 0, 0
}

// goBottom moves to the last unit and shows the end of the file.
func (m *Model) goBottom() {
	if m.fv == nil || len(m.fv.units) == 0 {
		return
	}
	m.noteScroll()
	if !m.visual {
		m.fv.sel = -1
	}
	m.fv.cursor = len(m.fv.units) - 1
	m.fv.top = m.maxTop()
}

// nextChange jumps to the start of the next (dir=1) or previous (dir=-1) run
// of changed lines.
func (m *Model) nextChange(dir int) bool {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return false
	}
	f := &fv.e.file
	startOf := func(i int) bool {
		return fv.units[i].isChange(f) && (i == 0 || !fv.units[i-1].isChange(f))
	}
	for i := fv.cursor + dir; i >= 0 && i < len(fv.units); i += dir {
		if startOf(i) {
			m.noteScroll()
			if !m.visual {
				fv.sel = -1
			}
			fv.cursor = i
			// Show the change with a little context above it.
			if d := int(fv.dlOf[i]); d < fv.top || d >= fv.top+m.bodyH()-scrolloff {
				fv.top = d - min(5, m.bodyH()/4)
			}
			m.clampTop()
			m.keepCursorVisible()
			return true
		}
	}
	return false
}

// hscroll moves the code sideways by n cells.
func (m *Model) hscroll(n int) {
	hx := max(m.hx+n, 0)
	if hx == m.hx || m.fv == nil {
		return
	}
	m.hx = hx
	m.fv.gen++
}

// noteScroll is called when the user moves: with reduced motion the static
// new-hunk marks end here.
func (m *Model) noteScroll() {
	if m.anim.freshOn && !m.t.Animated() {
		m.clearFresh()
	}
}

// unitAtLine returns the unit of a display line (-1 for blank and end rows).
func (m *Model) unitAtLine(di int) int {
	fv := m.fv
	if fv == nil || di < 0 || di >= len(fv.dl) {
		return -1
	}
	d := fv.dl[di]
	if d.kind == dlUnit || d.kind == dlBox {
		return int(d.unit)
	}
	return -1
}
