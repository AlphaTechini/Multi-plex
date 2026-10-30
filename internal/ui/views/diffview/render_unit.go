package diffview

import (
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// renderUnit returns the rendered row of unit ui at width dw. Plain rows are
// cached; rows under the cursor or selection, or carrying the new-hunk
// highlight, are drawn each frame.
func (m *Model) renderUnit(ui, dw int) string {
	fv := m.fv
	sel := m.isSelected(ui)
	fresh := m.freshAmount(ui)
	if !sel && fresh == 0 {
		if c := &fv.rc[ui]; c.gen == fv.gen && c.s != "" {
			return c.s
		}
		s := m.drawUnit(ui, dw, false, 0)
		fv.rc[ui] = rcEntry{s: s, gen: fv.gen}
		return s
	}
	return m.drawUnit(ui, dw, sel, fresh)
}

// isSelected reports whether unit ui is drawn as selected. The cursor and
// selection show only while the view has focus (or a comment is open).
func (m *Model) isSelected(ui int) bool {
	if !m.focused && m.cmp == nil {
		return false
	}
	fv := m.fv
	if fv.sel < 0 {
		return ui == fv.cursor
	}
	lo, hi := min(fv.sel, fv.cursor), max(fv.sel, fv.cursor)
	return ui >= lo && ui <= hi
}

// freshAmount returns the new-hunk highlight of a unit (0 when none).
func (m *Model) freshAmount(ui int) float64 {
	if !m.anim.freshOn || m.anim.fresh <= 0 {
		return 0
	}
	set := m.fresh[m.fv.e.path]
	if len(set) == 0 {
		return 0
	}
	f := &m.fv.e.file
	u := m.fv.units[ui]
	switch u.kind {
	case uHunk:
		if len(f.Hunks[u.hunk].Lines) > 0 {
			if _, ok := set[idOf(&f.Hunks[u.hunk].Lines[0])]; ok {
				return m.anim.fresh
			}
		}
		return 0
	case uFold:
		return 0
	}
	l, r := u.lines(f)
	for _, ln := range [2]*diff.Line{l, r} {
		if ln == nil {
			continue
		}
		if _, ok := set[idOf(ln)]; ok {
			return m.anim.fresh
		}
	}
	return 0
}

// drawUnit builds the row of a unit with kit.DiffLine.
func (m *Model) drawUnit(ui, dw int, sel bool, fresh float64) string {
	fv := m.fv
	t := m.t
	f := &fv.e.file
	u := fv.units[ui]
	switch u.kind {
	case uHunk:
		h := &f.Hunks[u.hunk]
		return kit.DiffLine(t, kit.DiffLineOptions{
			Width: dw, Kind: kit.DiffHunk, NumberWidth: fv.nw, Selected: sel,
			Header: hunkHeader(h), Spans: m.hunkSpans(h),
		})
	case uFold:
		return kit.DiffLine(t, kit.DiffLineOptions{
			Width: dw, Kind: kit.DiffFold, NumberWidth: fv.nw, Selected: sel,
			Folded: fv.rows[u.row].Collapsed,
		})
	case uPair:
		return m.drawPair(u, dw, sel, fresh)
	}
	ln := &f.Hunks[u.hunk].Lines[u.l]
	return kit.DiffLine(t, kit.DiffLineOptions{
		Width: dw, Kind: diffKind(ln), Old: ln.OldNo, New: ln.NewNo, NumberWidth: fv.nw,
		Spans: m.codeSpans(fv.tok, ln), Selected: sel, Highlight: fresh,
	})
}

// diffKind maps a line kind to the kit row kind.
func diffKind(ln *diff.Line) kit.DiffKind {
	switch ln.Kind {
	case diff.Add:
		return kit.DiffAdd
	case diff.Del:
		return kit.DiffDel
	}
	return kit.DiffContext
}

// hunkHeader formats "@@ -38,13 +38,15 @@".
func hunkHeader(h *diff.Hunk) string {
	return "@@ -" + strconv.Itoa(h.OldStart) + "," + strconv.Itoa(h.OldLines) +
		" +" + strconv.Itoa(h.NewStart) + "," + strconv.Itoa(h.NewLines) + " @@"
}

// hunkSpans are the reviewed mark and the muted enclosing declaration of a
// hunk header (the mark comes first so a long declaration cannot cut it).
func (m *Model) hunkSpans(h *diff.Hunk) []kit.Span {
	var spans []kit.Span
	if m.rev != nil && m.rev.hunkReviewed(m.fv.e.path, h) {
		spans = append(spans, kit.Span{Text: m.t.Glyphs.Check + " ", Fg: m.t.State.Done})
	}
	if h.Header != "" {
		spans = append(spans, kit.Span{Text: strings.ReplaceAll(h.Header, "\t", " "), Fg: m.t.Text.Muted})
	}
	return spans
}

// drawPair renders a side-by-side row: the old side on the left, the new
// side on the right, one divider column between them.
func (m *Model) drawPair(u unit, dw int, sel bool, fresh float64) string {
	fv := m.fv
	t := m.t
	lw := (dw - 1) / 2
	rw := dw - 1 - lw
	h := &fv.e.file.Hunks[u.hunk]
	side := func(idx int32, w int, selected bool, old bool) string {
		o := kit.DiffLineOptions{Width: w, Kind: kit.DiffContext, NumberWidth: fv.nw, Selected: selected}
		if idx >= 0 {
			ln := &h.Lines[idx]
			o.Kind = diffKind(ln)
			if old {
				o.Old = ln.OldNo
			} else {
				o.New = ln.NewNo
			}
			o.Spans = m.codeSpans(fv.tok, ln)
			o.Highlight = fresh
		}
		return kit.DiffLine(t, o)
	}
	div := "│"
	if !t.Glyphs.Unicode {
		div = "|"
	}
	return side(u.l, lw, sel, true) + kit.Backfill(paint(t.Border.Subtle, false, div), t.Surface.Base) + side(u.r, rw, false, false)
}
