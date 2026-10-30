package diffview

import (
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// railKind is the kind of a row of the file list.
type railKind uint8

// File list row kinds.
const (
	railBlank railKind = iota
	railDir
	railFile
)

// railItem is one row of the file list: a directory header, a file or a gap.
type railItem struct {
	kind railKind
	dir  string
	file int
}

// railCache caches the rendered file list.
type railCache struct {
	items []railItem
	ver   int
	built bool

	lines      []string
	key        railKey
	linesBuilt bool
}

// railKey is what the rendered rail depends on.
type railKey struct {
	w, ver, cur, focus int
}

// railItems returns the rows of the file list, grouped by directory.
func (m *Model) railItems() []railItem {
	rc := &m.railCache
	if rc.built && rc.ver == m.filesVer {
		return rc.items
	}
	items := make([]railItem, 0, len(m.files)*2)
	last := "\x00"
	for i, e := range m.files {
		if e.dir != last {
			if last != "\x00" {
				items = append(items, railItem{kind: railBlank})
			}
			items = append(items, railItem{kind: railDir, dir: e.dir})
			last = e.dir
		}
		items = append(items, railItem{kind: railFile, file: i})
	}
	rc.items, rc.ver, rc.built, rc.linesBuilt = items, m.filesVer, true, false
	return items
}

// railItemOf returns the index of a file's row in the list.
func (m *Model) railItemOf(file int) int {
	for i, it := range m.railItems() {
		if it.kind == railFile && it.file == file {
			return i
		}
	}
	return 0
}

// railLines renders every row of the list at width w. cur is the selected
// file; focused draws the accent bar (used by the overlay list).
func (m *Model) railLines(w, cur int, focused bool) []string {
	rc := &m.railCache
	items := m.railItems()
	f := 0
	if focused {
		f = 1
	}
	key := railKey{w: w, ver: m.filesVer, cur: cur, focus: f}
	if rc.linesBuilt && rc.key == key {
		return rc.lines
	}
	t := m.t
	lines := make([]string, len(items))
	blank := m.blankRow(w)
	for i, it := range items {
		switch it.kind {
		case railBlank:
			lines[i] = blank
		case railDir:
			dir := it.dir + "/"
			if it.dir == "" {
				dir = "./"
			}
			lines[i] = kit.Backfill(kit.Fit("  "+paint(t.Text.Muted, false, kit.TruncateLeft(dir, max(w-3, 1), t.Glyphs.Ellipsis)), w), t.Surface.Base)
		default:
			lines[i] = m.railFileRow(it.file, w, it.file == cur, focused)
		}
	}
	rc.lines, rc.key, rc.linesBuilt = lines, key, true
	return lines
}

// railFileRow renders one file of the list.
func (m *Model) railFileRow(i, w int, selected, focused bool) string {
	t := m.t
	e := m.files[i]
	reviewed := m.rev != nil && m.rev.fileReviewed(e)
	check := " "
	if reviewed {
		check = paint(t.State.Done, false, t.Glyphs.Check)
	}
	status := paint(statusColor(m, e), true, e.letter())
	nameC := t.Text.Primary
	switch {
	case selected:
		nameC = t.Text.Strong
	case reviewed:
		nameC = t.Text.Muted
	}
	counts := ""
	if e.file.Added > 0 {
		counts += paint(t.Diff.AddSign, false, "+"+strconv.Itoa(e.file.Added))
	}
	if e.file.Deleted > 0 {
		if counts != "" {
			counts += " "
		}
		counts += paint(t.Diff.DelSign, false, t.Glyphs.Minus+strconv.Itoa(e.file.Deleted))
	}
	return kit.Row(t, kit.RowOptions{
		Width: w, Selected: selected, Focused: focused,
		Left:  " " + check + " " + status + "  " + paint(nameC, selected, e.name),
		Right: counts,
	})
}

// railView renders the file rail at w × h: a header, the list (scrolled to
// keep the current file in view) and the footer hints.
func (m *Model) railView(w, h int) []string {
	t := m.t
	out := make([]string, 0, h)
	blank := m.blankRow(w)
	meta := strconv.Itoa(m.rev.reviewedCount(m.files)) + "/" + strconv.Itoa(len(m.files)) + " reviewed"
	out = append(out, blank, kit.Backfill(kit.Section(t, kit.SectionOptions{Width: w, Label: "Files", Meta: meta}), t.Surface.Base), blank)

	listH := max(h-5, 0)
	items := m.railLines(w, m.fi, false)
	m.railTop = scrollInto(m.railTop, m.railItemOf(m.fi), listH, len(items))
	for i := 0; i < listH; i++ {
		if j := m.railTop + i; j < len(items) {
			out = append(out, items[j])
		} else {
			out = append(out, blank)
		}
	}
	hints := kit.KeyHints(t, []kit.Hint{{Keys: "[", Label: "prev"}, {Keys: "]", Label: "next"}, {Keys: "v", Label: "reviewed"}}, 0)
	out = append(out, kit.Backfill(kit.Fit("  "+hints, w), t.Surface.Base), blank)
	for len(out) < h {
		out = append(out, blank)
	}
	return out[:h]
}

// scrollInto returns the top offset of a window of h rows over n rows that
// shows row at.
func scrollInto(top, at, h, n int) int {
	if h <= 0 {
		return 0
	}
	if at < top {
		top = at
		// Leave the directory header above the first file visible.
		top = max(top-1, 0)
	}
	if at >= top+h {
		top = at - h + 1
	}
	return min(max(top, 0), max(n-h, 0))
}

// railFileAt returns the file index shown on rail row y (counted from the
// top of the view), or -1.
func (m *Model) railFileAt(y int) int {
	i := m.railTop + y - 3
	items := m.railItems()
	if y < 3 || i < 0 || i >= len(items) || items[i].kind != railFile {
		return -1
	}
	return items[i].file
}

// blankRow returns a painted blank row of width w.
func (m *Model) blankRow(w int) string {
	if r, ok := m.blank[w]; ok {
		return r
	}
	if m.blank == nil || len(m.blank) > 8 {
		m.blank = map[int]string{}
	}
	r := kit.Backfill(strings.Repeat(" ", max(w, 0)), m.t.Surface.Base)
	m.blank[w] = r
	return r
}
