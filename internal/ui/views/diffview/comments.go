package diffview

import (
	"sort"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// placedComment is a comment positioned in the current file.
type placedComment struct {
	c core.Comment
	// start and end are the comment's lines on the new side now (after
	// re-anchoring); 0 for a comment on deleted lines.
	start, end int
	// oldSide marks a comment on deleted lines.
	oldSide bool
	// outdated means the anchored text is gone; the original snippet is shown.
	outdated bool
	// changed means a sent comment's lines changed since.
	changed bool
	// unit is the unit the box hangs under; -1 puts it before the end marker.
	unit int
}

// anchorOf rebuilds the diff anchor stored in a comment. Context holds the
// lines around the snippet and the snippet itself: up to three before it,
// then the snippet, then up to three after.
func anchorOf(c core.Comment) diff.Anchor {
	a := diff.Anchor{Path: c.Path, Start: c.NewStart, End: c.NewEnd, Snippet: c.Snippet}
	if c.Context == "" || c.NewStart < 1 {
		return a
	}
	ctx := strings.Split(c.Context, "\n")
	n := strings.Count(c.Snippet, "\n") + 1
	before := min(diff.AnchorContext, c.NewStart-1)
	if before+n <= len(ctx) {
		a.Before = ctx[:before]
		a.After = ctx[before+n:]
	}
	return a
}

// freshContent returns the content of the current file when it was read for
// the current version of the diff.
func (m *Model) freshContent(e *fileEntry) ([]string, bool) {
	c := m.contents[e.path]
	if c == nil || c.binary || c.hash != e.hash {
		return nil, false
	}
	return c.lines, true
}

// placeComments positions the comments of the current file: it re-anchors
// them against the file's content (diff.Relocate), marks the ones whose text
// is gone as outdated and finds the unit each box hangs under. Comments whose
// range moved are queued for saving in m.saves.
func (m *Model) placeComments() {
	fv := m.fv
	fv.placed = fv.placed[:0]
	m.saves = m.saves[:0]
	e := fv.e
	var mine []int
	for i := range m.comments {
		if m.comments[i].Path == e.path {
			mine = append(mine, i)
		}
	}
	if len(mine) == 0 {
		return
	}
	type nu struct{ no, unit int }
	var newIdx []nu
	delIdx := map[int]int{}
	for ui, u := range fv.units {
		l, r := u.lines(&e.file)
		if r != nil && r.NewNo > 0 {
			newIdx = append(newIdx, nu{r.NewNo, ui})
		}
		if l != nil && l.Kind == diff.Del {
			delIdx[l.OldNo] = ui
		}
	}
	lookupNew := func(no int) int {
		i := sort.Search(len(newIdx), func(i int) bool { return newIdx[i].no >= no })
		if i < len(newIdx) {
			return newIdx[i].unit
		}
		return -1
	}
	content, have := m.freshContent(e)
	for _, ci := range mine {
		c := m.comments[ci]
		p := placedComment{c: c, start: c.NewStart, end: c.NewEnd, unit: -1}
		snip := strings.Split(c.Snippet, "\n")
		lastSnip := strings.TrimSuffix(snip[len(snip)-1], "\r")
		placedOld := false
		if c.OldStart > 0 {
			if ui, ok := delIdx[c.OldEnd]; ok {
				if l, _ := fv.units[ui].lines(&e.file); l != nil && l.Text == lastSnip {
					p.oldSide, p.start, p.end, p.unit, placedOld = true, 0, 0, ui, true
				}
			}
		}
		if !placedOld {
			switch {
			case have && c.NewStart >= 1:
				a := anchorOf(c)
				s, en, ok := diff.Relocate(a, content)
				if !ok {
					p.outdated = true
					break
				}
				p.start, p.end, p.unit = s, en, lookupNew(en)
				if c.Status == core.CommentSent {
					p.changed = diff.Changed(a, content)
				}
				if (s != c.NewStart || en != c.NewEnd) && (c.Status == core.CommentDraft || c.Status == core.CommentQueued) {
					moved := c
					moved.NewStart, moved.NewEnd = s, en
					na := diff.NewAnchor(c.Path, s, en, content)
					moved.Context = strings.Join(append(append(append([]string(nil), na.Before...), strings.Split(na.Snippet, "\n")...), na.After...), "\n")
					m.comments[ci] = moved
					p.c = moved
					m.saves = append(m.saves, moved)
				}
			case c.NewStart >= 1:
				p.unit = lookupNew(c.NewEnd)
			default:
				p.outdated = true
			}
		}
		fv.placed = append(fv.placed, p)
	}
}

// buildBoxes renders the comment boxes of the current file (and the
// composer) at the current width.
func (m *Model) buildBoxes() {
	fv := m.fv
	fv.boxes = fv.boxes[:0]
	fv.after = nil
	fv.tail = nil
	if fv.w <= 0 {
		return
	}
	add := func(unit int, lines []string, placed int) {
		fv.boxes = append(fv.boxes, box{lines: lines, placed: placed})
		bi := len(fv.boxes) - 1
		if unit < 0 || unit >= len(fv.units) {
			fv.tail = append(fv.tail, bi)
			return
		}
		if fv.after == nil {
			fv.after = map[int][]int{}
		}
		fv.after[unit] = append(fv.after[unit], bi)
	}
	for i := range fv.placed {
		p := &fv.placed[i]
		if m.cmp != nil && m.cmp.editID != "" && p.c.ID == m.cmp.editID {
			continue
		}
		add(p.unit, m.commentLines(p), i)
	}
	if m.cmp != nil {
		add(m.cmp.unit, m.composerLines(), -1)
	}
}

// dash is the range separator of line labels.
func (m *Model) dash() string {
	if m.t.Glyphs.Unicode {
		return "–"
	}
	return "-"
}

// rangeLabel formats a line range ("L44–L46", "L12").
func (m *Model) rangeLabel(start, end int) string {
	if start <= 0 {
		return ""
	}
	if end <= start {
		return "L" + itoa(start)
	}
	return "L" + itoa(start) + m.dash() + "L" + itoa(end)
}

// commentLines renders one placed comment as a box.
func (m *Model) commentLines(p *placedComment) []string {
	fv := m.fv
	status := string(p.c.Status)
	rng := m.rangeLabel(p.start, p.end)
	body := p.c.Body
	switch {
	case p.outdated:
		status = "outdated"
		rng = m.rangeLabel(p.c.NewStart, p.c.NewEnd)
		body += "\n\nOriginal lines:\n" + quoteSnippet(p.c.Snippet)
	case p.oldSide:
		rng = m.rangeLabel(p.c.OldStart, p.c.OldEnd) + " removed"
	case p.changed:
		rng += " " + m.t.Glyphs.Dot + " lines changed"
	}
	return kit.Lines(kit.CommentBox(m.t, kit.CommentBoxOptions{
		Width: fv.w, Indent: kit.DiffGutterWidth(fv.nw) - 2,
		Author: "You", Status: status, Range: rng, Body: body,
	}))
}

// quoteSnippet shows up to four lines of a snippet for an outdated comment.
func quoteSnippet(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 4 {
		lines = append(lines[:4], "…")
	}
	for i, l := range lines {
		lines[i] = "  " + strings.ReplaceAll(l, "\t", "    ")
	}
	return strings.Join(lines, "\n")
}

// queuedCount is the number of comments waiting to be sent.
func (m *Model) queuedCount() int {
	n := 0
	for i := range m.comments {
		if m.comments[i].Status == core.CommentQueued {
			n++
		}
	}
	return n
}

// commentAt returns the placed comment hanging under a unit, if any.
func (m *Model) commentAt(unit int) *placedComment {
	fv := m.fv
	if fv == nil {
		return nil
	}
	for i := range fv.placed {
		if fv.placed[i].unit == unit {
			return &fv.placed[i]
		}
	}
	return nil
}
