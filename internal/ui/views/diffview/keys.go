package diffview

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// onKey handles a key press. While a comment is being typed every key goes
// to the comment box.
func (m *Model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.cmp != nil {
		return m.composeKey(msg)
	}
	if m.overlay {
		return m.overlayKey(msg)
	}
	switch s := msg.String(); s {
	case "]":
		return m.stepFile(1)
	case "[":
		return m.stepFile(-1)
	case "f":
		m.toggleFiles()
		return nil
	case "v", "r":
		return m.toggleReviewed()
	case "s":
		return m.toggleSideBySide()
	case "S", "ctrl+s":
		return m.sendQueuedCmd()
	}
	fv := m.fv
	if fv == nil {
		return nil
	}
	if !m.hasRows(fv.e) {
		if msg.String() == "enter" && fv.e.huge && !m.forceHuge[fv.e.path] {
			return m.openHuge()
		}
		return nil
	}
	switch msg.String() {
	case "j", "down":
		m.moveCursor(1, false)
	case "k", "up":
		m.moveCursor(-1, false)
	case "J", "shift+j", "shift+down":
		m.moveCursor(1, true)
	case "K", "shift+k", "shift+up":
		m.moveCursor(-1, true)
	case "V":
		m.visual = !m.visual
		if m.visual && fv.sel < 0 {
			fv.sel = fv.cursor
		} else if !m.visual {
			fv.sel = -1
		}
	case "esc":
		m.visual = false
		fv.sel = -1
	case "ctrl+d", "pgdown":
		m.halfPage(1)
	case "ctrl+u", "pgup":
		m.halfPage(-1)
	case "g", "home":
		m.goTop()
	case "G", "end":
		m.goBottom()
	case "n":
		m.nextChange(1)
	case "N":
		m.nextChange(-1)
	case "h", "left":
		m.hscroll(-hscrollStep)
	case "l", "right":
		m.hscroll(hscrollStep)
	case "enter":
		if fv.units[fv.cursor].kind == uFold {
			return m.expandFold(fv.cursor)
		}
		return m.startCompose()
	case "c":
		return m.startCompose()
	case "e":
		if p := m.commentForCursor(); p != nil {
			return m.startEdit(p)
		}
	case "d":
		return m.deleteCursorComment()
	case "y":
		return m.copyCmd()
	case "x":
		m.toggleHunkReviewed()
	}
	return nil
}

// toggleFiles shows or hides the file rail, or opens the file list overlay
// when the view is too narrow for the rail.
func (m *Model) toggleFiles() {
	if m.w >= railMinWidth {
		m.railHidden = !m.railHidden
		if m.fv != nil {
			m.fv.w = 0 // the diff width changes; Render re-lays out
		}
		return
	}
	if len(m.files) == 0 {
		return
	}
	m.overlay = true
	m.overlayCur = max(m.fi, 0)
}

// overlayKey handles keys while the file list overlay is open.
func (m *Model) overlayKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "f", "q":
		m.overlay = false
	case "j", "down", "ctrl+n":
		m.overlayCur = min(m.overlayCur+1, len(m.files)-1)
	case "k", "up", "ctrl+p":
		m.overlayCur = max(m.overlayCur-1, 0)
	case "g", "home":
		m.overlayCur = 0
	case "G", "end":
		m.overlayCur = len(m.files) - 1
	case "enter":
		i := m.overlayCur
		m.overlay = false
		if i != m.fi {
			return m.openFile(i, true)
		}
	case "]":
		m.overlay = false
		return m.stepFile(1)
	case "[":
		m.overlay = false
		return m.stepFile(-1)
	}
	return nil
}

// toggleReviewed flips the current file's reviewed mark.
func (m *Model) toggleReviewed() tea.Cmd {
	if m.fv == nil || m.rev == nil {
		return nil
	}
	m.rev.toggleFile(m.fv.e)
	m.filesVer++
	m.fv.gen++
	return nil
}

// toggleHunkReviewed flips the reviewed mark of the hunk under the cursor.
func (m *Model) toggleHunkReviewed() {
	fv := m.fv
	if fv == nil || m.rev == nil || len(fv.units) == 0 {
		return
	}
	u := fv.units[fv.cursor]
	if u.kind == uFold {
		return
	}
	m.rev.toggleHunk(fv.e.path, &fv.e.file.Hunks[u.hunk])
	fv.gen++
}

// toggleSideBySide switches between unified and side-by-side.
func (m *Model) toggleSideBySide() tea.Cmd {
	fv := m.fv
	if !m.sbsAllowed() {
		return toastCmd(kit.ToneInfo, "Side-by-side needs a wider terminal",
			"It is available from 200 columns for the main view.")
	}
	m.sbs = !m.sbs
	if fv == nil {
		return nil
	}
	pos := m.capturePos()
	fv.sbs = m.sbs
	m.rebuild()
	m.restorePos(pos)
	return nil
}

// openHuge opens a file flagged as very large.
func (m *Model) openHuge() tea.Cmd {
	fv := m.fv
	m.forceHuge[fv.e.path] = true
	m.rebuild()
	m.initialCursor()
	return m.readContentCmd(fv.e)
}

// commentForCursor returns the comment on the cursor line: one hanging under
// it or covering its line number.
func (m *Model) commentForCursor() *placedComment {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return nil
	}
	if p := m.commentAt(fv.cursor); p != nil {
		return p
	}
	_, r := fv.units[fv.cursor].lines(&fv.e.file)
	if r == nil {
		return nil
	}
	for i := range fv.placed {
		p := &fv.placed[i]
		if !p.oldSide && !p.outdated && p.start <= r.NewNo && r.NewNo <= p.end {
			return p
		}
	}
	return nil
}

// deleteCursorComment deletes the comment on the cursor line.
func (m *Model) deleteCursorComment() tea.Cmd {
	p := m.commentForCursor()
	if p == nil {
		return nil
	}
	id := p.c.ID
	if id == "" {
		return nil // still being saved
	}
	for i := range m.comments {
		if m.comments[i].ID == id {
			m.comments = append(m.comments[:i], m.comments[i+1:]...)
			break
		}
	}
	pos := m.capturePos()
	m.rebuild()
	m.restorePos(pos)
	return tea.Batch(m.deleteCommentCmd(id), toastCmd(kit.ToneInfo, "Comment deleted", ""))
}

// copyCmd copies "path:Lx-Ly" and the selected lines to the clipboard
// (OSC 52 through Bubble Tea).
func (m *Model) copyCmd() tea.Cmd {
	a, ok := m.selectionAnchor()
	if !ok {
		return nil
	}
	ref := a.path + ":" + plainRange(a.newStart, a.newEnd)
	if a.deletionOnly {
		ref = a.path + ":" + plainRange(a.oldStart, a.oldEnd) + " (removed)"
	}
	return tea.Batch(
		tea.SetClipboard(ref+"\n"+a.snippet),
		toastCmd(kit.ToneSuccess, "Copied "+a.label, ref),
	)
}

// plainRange formats "L12-L30" the way the message to the agent does.
func plainRange(start, end int) string {
	if end > start {
		return "L" + itoa(start) + "-L" + itoa(end)
	}
	return "L" + itoa(start)
}
