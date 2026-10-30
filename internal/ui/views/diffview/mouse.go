package diffview

import (
	tea "charm.land/bubbletea/v2"
)

// onClick handles a left click: a file in the rail, a line of the diff (shift
// extends the selection) or a folded region (expands it).
func (m *Model) onClick(msg tea.MouseClickMsg) tea.Cmd {
	mm := msg.Mouse()
	if mm.Button != tea.MouseLeft || m.cmp != nil {
		return nil
	}
	if m.overlay {
		return m.overlayClick(mm.X, mm.Y)
	}
	x, y := mm.X, mm.Y
	if m.railShown() {
		if x < railWidth {
			if i := m.railFileAt(y); i >= 0 && i != m.fi {
				return m.openFile(i, true)
			}
			return nil
		}
	}
	fv := m.fv
	if fv == nil || y < bodyTop || !m.hasRows(fv.e) {
		return nil
	}
	ui := m.unitAtLine(fv.top + y - bodyTop)
	if ui < 0 || ui >= len(fv.units) {
		return nil
	}
	m.noteScroll()
	if fv.units[ui].kind == uFold {
		fv.cursor, fv.sel = ui, -1
		return m.expandFold(ui)
	}
	if mm.Mod.Contains(tea.ModShift) {
		if fv.sel < 0 {
			fv.sel = fv.cursor
		}
		fv.cursor = ui
	} else {
		fv.cursor, fv.sel = ui, -1
		m.visual = false
		m.dragging = true
	}
	return nil
}

// onDrag extends the selection while the left button is held.
func (m *Model) onDrag(msg tea.MouseMotionMsg) tea.Cmd {
	fv := m.fv
	if !m.dragging || fv == nil || m.cmp != nil || msg.Mouse().Button != tea.MouseLeft {
		return nil
	}
	y := msg.Mouse().Y
	// Dragging past the edges scrolls.
	switch {
	case y < bodyTop:
		m.scrollBy(-1)
		y = bodyTop
	case y >= m.h:
		m.scrollBy(1)
		y = m.h - 1
	}
	ui := m.unitAtLine(fv.top + y - bodyTop)
	if ui < 0 || ui >= len(fv.units) || ui == fv.cursor {
		return nil
	}
	if fv.sel < 0 {
		fv.sel = fv.cursor
	}
	fv.cursor = ui
	return nil
}

// overlayClick selects a file in the file list overlay.
func (m *Model) overlayClick(x, y int) tea.Cmd {
	w := min(overlayWidth, m.diffWidth()-2)
	h := min(m.h-2, len(m.railItems())+4)
	// The overlay is a framed panel at (2, 2).
	if x < 2 || x >= 2+w+2 || y < 2 || y >= 2+h {
		m.overlay = false
		return nil
	}
	row := y - 3
	if row < 0 || row >= h-2 {
		return nil
	}
	top := scrollInto(0, m.railItemOf(m.overlayCur), h-2, len(m.railItems()))
	items := m.railItems()
	if i := top + row; i < len(items) && items[i].kind == railFile {
		m.overlay = false
		if items[i].file != m.fi {
			return m.openFile(items[i].file, true)
		}
	}
	return nil
}

// onWheel scrolls the diff (or the rail) with the mouse wheel.
func (m *Model) onWheel(msg tea.MouseWheelMsg) tea.Cmd {
	mm := msg.Mouse()
	if m.cmp != nil {
		return nil
	}
	switch mm.Button {
	case tea.MouseWheelUp, tea.MouseWheelDown:
		d := scrollStep
		if mm.Button == tea.MouseWheelUp {
			d = -scrollStep
		}
		if m.railShown() && mm.X < railWidth {
			m.railTop = max(m.railTop+d, 0)
			return nil
		}
		m.scrollBy(d)
	case tea.MouseWheelLeft:
		m.hscroll(-hscrollStep)
	case tea.MouseWheelRight:
		m.hscroll(hscrollStep)
	}
	return nil
}
