package diffview

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// needsContent reports whether the file's working-tree content is useful:
// to expand folded regions, size the trailing gap and re-anchor comments.
func needsContent(e *fileEntry) bool {
	return len(e.file.Hunks) > 0 && !e.file.Binary && e.file.Status != vcs.Deleted
}

// readContentCmd reads the current file's content unless it is cached.
func (m *Model) readContentCmd(e *fileEntry) tea.Cmd {
	if !needsContent(e) {
		return nil
	}
	if c := m.contents[e.path]; c != nil && c.hash == e.hash {
		return nil
	}
	if h, busy := m.reading[e.path]; busy && h == e.hash {
		return nil
	}
	m.reading[e.path] = e.hash
	be, id, gen, path, hash := m.env.Backend, m.session.ID, m.gen, e.path, e.hash
	return func() tea.Msg {
		ctx, cancel := ctxTimeout()
		defer cancel()
		res, err := be.ReadFile(ctx, id, path, "")
		if err != nil {
			return contentMsg{gen: gen, path: path, hash: hash, err: err}
		}
		return contentMsg{gen: gen, path: path, hash: hash, lines: diff.SplitLines(res.Content), binary: res.Binary}
	}
}

// onContent stores a file's content and, for the file on screen, applies the
// regions the user had expanded, sizes the trailing gap and re-anchors the
// file's comments.
func (m *Model) onContent(msg contentMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil
	}
	delete(m.reading, msg.path)
	if msg.err != nil {
		// Without content the diff still works; only expanding is unavailable.
		return nil
	}
	m.storeContent(msg.path, &content{lines: msg.lines, binary: msg.binary, hash: msg.hash})
	fv := m.fv
	if fv == nil || fv.e.path != msg.path {
		return nil
	}
	if fv.e.hash != msg.hash {
		return m.readContentCmd(fv.e) // the file changed again meanwhile
	}
	pos := m.capturePos()
	m.reapplyGaps(fv.e, msg.lines)
	m.rebuild()
	m.restorePos(pos)
	fv.gen++
	return m.flushSaves()
}

// storeContent caches a file's content, evicting the oldest beyond the cap.
func (m *Model) storeContent(path string, c *content) {
	if _, ok := m.contents[path]; !ok {
		m.order = append(m.order, path)
	}
	m.contents[path] = c
	for len(m.order) > maxCachedContents {
		delete(m.contents, m.order[0])
		m.order = m.order[1:]
	}
}

// reapplyInner marks again the in-hunk context runs the user expanded, after
// a re-parse (the parse forgets Line.Expanded).
func (m *Model) reapplyInner(e *fileEntry) {
	set := m.innerExp[e.path]
	if len(set) == 0 {
		return
	}
	for hi := range e.file.Hunks {
		lines := e.file.Hunks[hi].Lines
		for li := range lines {
			if lines[li].Kind == diff.Context && set[lines[li].NewNo] {
				lines[li].Expanded = true
			}
		}
	}
}

// gapBounds returns the new-side lines [from, to) of gap g of the file.
func gapBounds(f *diff.File, g, total int) (from, to int) {
	from = 1
	if g > 0 {
		p := &f.Hunks[g-1]
		from = hunkFirst(p.NewStart, p.NewLines) + p.NewLines
	}
	if g < len(f.Hunks) {
		h := &f.Hunks[g]
		to = hunkFirst(h.NewStart, h.NewLines)
	} else {
		to = total + 1
	}
	return from, to
}

// hunkFirst is the first line a hunk range covers (git writes the line
// before an empty range).
func hunkFirst(start, n int) int {
	if n == 0 {
		return start + 1
	}
	return start
}

// reapplyGaps expands again the gaps between hunks the user had opened, on a
// freshly parsed file. It runs once per parsed entry.
func (m *Model) reapplyGaps(e *fileEntry, lines []string) {
	if m.appliedGp[e] {
		return
	}
	m.appliedGp[e] = true
	for _, r := range m.expanded[e.path] {
		for g := 0; g <= len(e.file.Hunks); g++ {
			from, to := gapBounds(&e.file, g, len(lines))
			if from <= r.from && r.to <= to {
				diff.ExpandGap(&e.file, g, lines)
				break
			}
		}
	}
}

// expandFold opens the folded region of a fold unit.
func (m *Model) expandFold(ui int) tea.Cmd {
	fv := m.fv
	u := fv.units[ui]
	row := fv.rows[u.row]
	e := fv.e
	pos := m.capturePos()
	revealed := 0 // first new-side line that becomes visible
	if row.Gap > len(e.file.Hunks) {
		// A run of context inside a hunk: flag one of its lines.
		h := &e.file.Hunks[row.Hunk]
		if row.Line >= len(h.Lines) {
			return nil
		}
		h.Lines[row.Line].Expanded = true
		revealed = h.Lines[row.Line].NewNo
		if m.innerExp[e.path] == nil {
			m.innerExp[e.path] = map[int]bool{}
		}
		m.innerExp[e.path][h.Lines[row.Line].NewNo] = true
	} else {
		c := m.contents[e.path]
		if c == nil || c.hash != e.hash {
			return tea.Batch(m.readContentCmd(e), toastCmd(kit.ToneInfo, "Loading the file", "Press enter again in a moment."))
		}
		from, to := gapBounds(&e.file, row.Gap, len(c.lines))
		if !diff.ExpandGap(&e.file, row.Gap, c.lines) {
			return nil
		}
		m.expanded[e.path] = append(m.expanded[e.path], gapRange{from, to})
		revealed = from
	}
	m.rebuild()
	m.restorePos(pos)
	if revealed > 0 {
		fv.cursor = m.findUnitByNew(revealed)
		fv.sel = -1
		m.keepCursorVisible()
	}
	return nil
}
