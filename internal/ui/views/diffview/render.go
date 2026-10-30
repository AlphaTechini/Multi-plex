package diffview

import (
	"image/color"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// eofCache caches the end-of-file marker row.
type eofCache struct {
	row  string
	w    int
	next string
	n    int
}

// statusColor returns the rail colour of a file's status letter.
func statusColor(m *Model, e *fileEntry) color.Color {
	switch e.letter() {
	case "A":
		return m.t.Diff.AddSign
	case "D":
		return m.t.Diff.DelSign
	case "R":
		return m.t.Diff.Renamed
	}
	return m.t.Diff.Modified
}

// Render draws the view at exactly width × height cells. It builds strings
// only for the rows on screen and reuses cached rows; it does no I/O.
func (m *Model) Render(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	m.w, m.h = width, height
	if s, ok := m.renderState(width, height); ok {
		return s
	}

	dw := m.diffWidth()
	fv := m.fv
	if fv.w != dw || fv.sbs != (m.sbs && m.sbsAllowed()) {
		m.relayout(dw)
	}
	m.clampTop()

	area := m.diffArea(dw, height)
	if m.overlay {
		area = m.overlayOver(area, dw, height)
	}
	if !m.railShown() {
		return strings.Join(area, "\n")
	}
	rail := m.railView(railWidth, height)
	sep := kit.Backfill(paint(m.t.Border.Subtle, false, "│"), m.t.Surface.Base)
	if !m.t.Glyphs.Unicode {
		sep = kit.Backfill(paint(m.t.Border.Subtle, false, "|"), m.t.Surface.Base)
	}
	out := make([]string, height)
	for i := range out {
		out[i] = rail[i] + sep + area[i]
	}
	return strings.Join(out, "\n")
}

// relayout adapts the file view to a new diff width or display mode.
func (m *Model) relayout(dw int) {
	fv := m.fv
	pos := m.capturePos()
	fv.w = dw
	if want := m.sbs && m.sbsAllowed(); fv.sbs != want {
		fv.sbs = want
		m.rebuild()
	} else {
		m.layout()
	}
	m.restorePos(pos)
}

// diffArea renders the banner, the hint row and the body, each dw wide.
func (m *Model) diffArea(dw, height int) []string {
	t := m.t
	e := m.fv.e
	out := make([]string, 0, height)
	out = append(out, kit.Banner(t, kit.BannerOptions{
		Width: dw, Path: e.path, Index: m.fi + 1, Total: len(m.files),
		Status: e.status(), Added: e.file.Added, Removed: e.file.Deleted,
		Reviewed: m.rev != nil && m.rev.fileReviewed(e), Settle: m.bannerSettle(),
	}))
	if m.railShown() {
		out = append(out, m.blankRow(dw))
	} else {
		out = append(out, kit.Backfill(kit.Fit("  "+kit.KeyHints(t, m.hintRow(), 0), dw), t.Surface.Base))
	}
	bh := max(height-bodyTop, 0)
	if title, body := m.fileNote(e); title != "" {
		out = append(out, kit.Lines(kit.EmptyState(t, kit.EmptyStateOptions{Width: dw, Height: bh, Title: title, Body: body}))...)
	} else {
		out = append(out, m.bodyLines(dw, bh)...)
	}
	out = out[:min(len(out), height)]
	if amt := m.fadeAmount(); amt > 0 && len(out) > 1 {
		rest := kit.Dim(t, strings.Join(out[1:], "\n"), t.Surface.Base, amt)
		out = append(out[:1], kit.Lines(rest)...)
	}
	return out
}

// hintRow is the row of key hints shown under the banner when the rail is
// collapsed (docs/16).
func (m *Model) hintRow() []kit.Hint {
	return []kit.Hint{
		{Keys: "f", Label: "files"},
		{Keys: "[", Label: "prev file"},
		{Keys: "]", Label: "next file"},
		{Keys: "n", Label: "next change"},
		{Keys: "c", Label: "comment"},
	}
}

// bodyLines renders the visible display lines.
func (m *Model) bodyLines(dw, bh int) []string {
	fv := m.fv
	out := make([]string, bh)
	for i := range out {
		di := fv.top + i
		if di >= len(fv.dl) {
			out[i] = m.blankRow(dw)
			continue
		}
		d := fv.dl[di]
		switch d.kind {
		case dlUnit:
			out[i] = m.renderUnit(int(d.unit), dw)
		case dlBox:
			out[i] = fv.boxes[d.box].lines[d.sub]
		case dlEOF:
			out[i] = m.eofRow(dw)
		default:
			out[i] = m.blankRow(dw)
		}
	}
	return out
}

// eofRow renders the end-of-file marker; it names the next file and never
// leads into it.
func (m *Model) eofRow(dw int) string {
	next := ""
	if n := m.nextFileEntry(); n != nil {
		next = n.path
	}
	c := &m.eofCache
	if c.row != "" && c.w == dw && c.next == next && c.n == len(m.files) {
		return c.row
	}
	c.row = kit.FileEnd(m.t, kit.FileEndOptions{Width: dw, Next: next, Keys: "]", Total: len(m.files)})
	c.w, c.next, c.n = dw, next, len(m.files)
	return c.row
}

// overlayOver draws the file list overlay over the diff area (used when the
// view is too narrow for the rail).
func (m *Model) overlayOver(area []string, dw, height int) []string {
	t := m.t
	w := min(overlayWidth, dw-2)
	if w < 12 {
		return area
	}
	items := m.railItems()
	h := min(height-2, len(items)+4)
	if h < 5 {
		return area
	}
	listH := h - 2
	lines := m.railLines(w, m.overlayCur, true)
	top := scrollInto(0, m.railItemOf(m.overlayCur), listH, len(lines))
	rows := make([]string, 0, listH)
	for i := 0; i < listH; i++ {
		if j := top + i; j < len(lines) {
			rows = append(rows, lines[j])
		}
	}
	meta := itoa(m.rev.reviewedCount(m.files)) + "/" + itoa(len(m.files)) + " reviewed"
	p := kit.Panel(t, kit.PanelOptions{
		Width: w + 2, Height: h, Title: "Files", Meta: meta, Focused: true, Flush: true,
		Footer:  kit.KeyHints(t, []kit.Hint{{Keys: "enter", Label: "open"}, {Keys: "esc", Label: "close"}}, 0),
		Content: strings.Join(rows, "\n"),
	})
	out := kit.Composite(strings.Join(area, "\n"), p, 2, 2)
	return kit.Lines(out)
}
