package diffview

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// leaveFile remembers the position in the current file.
func (m *Model) leaveFile() {
	if m.fv == nil || m.mem == nil {
		return
	}
	m.mem.positions[m.fv.e.path] = m.capturePos()
	m.mem.lastPath = m.fv.e.path
}

// openFile shows file idx. animate plays the banner tint and content fade
// (only with animation on). The position in the file is restored when the
// file was open before.
func (m *Model) openFile(idx int, animate bool) tea.Cmd {
	if idx < 0 || idx >= len(m.files) {
		return nil
	}
	m.leaveFile()
	m.fi = idx
	e := m.files[idx]
	m.cmp = nil
	m.hx = 0
	if !m.t.Animated() {
		m.clearFresh()
	}
	fv := &fileView{e: e, tok: m.tokens(e.path), sel: -1, w: m.diffWidth()}
	fv.sbs = m.sbs && m.sbsAllowed()
	m.fv = fv
	m.rebuild()
	if p, ok := m.mem.position(e.path); ok {
		m.restorePos(p)
	} else {
		m.initialCursor()
	}
	if animate {
		m.startSwitch()
	}
	m.overlay = false
	fv.gen++
	return tea.Batch(m.readContentCmd(e), m.flushSaves())
}

// position returns the remembered position in a file.
func (s *sessionMem) position(path string) (posState, bool) {
	if s == nil {
		return posState{}, false
	}
	p, ok := s.positions[path]
	return p, ok
}

// initialCursor puts the cursor on the first changed line when it is on the
// first screen, else on the first unit.
func (m *Model) initialCursor() {
	fv := m.fv
	fv.cursor, fv.sel, fv.top = 0, -1, 0
	for i, u := range fv.units {
		if u.isChange(&fv.e.file) {
			if int(fv.dlOf[i]) < m.bodyH()-2 {
				fv.cursor = i
			}
			return
		}
	}
	if len(fv.units) > 1 && fv.units[0].kind == uFold {
		fv.cursor = 1
	}
}

// stepFile moves to the next (d=1) or previous (d=-1) file. It never wraps.
func (m *Model) stepFile(d int) tea.Cmd {
	if len(m.files) == 0 {
		return nil
	}
	i := m.fi + d
	if i < 0 || i >= len(m.files) || i == m.fi {
		return nil
	}
	return m.openFile(i, true)
}

// onOpen opens a file (and line) the shell asked for.
func (m *Model) onOpen(msg view.SwitchModeMsg) tea.Cmd {
	if msg.Path == "" || len(m.files) == 0 {
		return nil
	}
	i := slices.IndexFunc(m.files, func(e *fileEntry) bool { return e.path == msg.Path })
	if i < 0 {
		return nil
	}
	var cmd tea.Cmd
	if i != m.fi {
		cmd = m.openFile(i, true)
	}
	if msg.Line > 0 && m.fv != nil && len(m.fv.units) > 0 {
		m.fv.cursor, m.fv.sel = m.findUnitByNew(msg.Line), -1
		m.keepCursorVisible()
	}
	return cmd
}

// nextFileEntry returns the file after the current one, or nil.
func (m *Model) nextFileEntry() *fileEntry {
	if m.fi >= 0 && m.fi+1 < len(m.files) {
		return m.files[m.fi+1]
	}
	return nil
}
