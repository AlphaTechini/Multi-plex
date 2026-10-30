package diffview

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/diff"
)

// anchorInfo describes the lines under the selection in the terms a comment
// stores (docs/07 "Anchoring").
type anchorInfo struct {
	path                string
	newStart, newEnd    int
	oldStart, oldEnd    int
	snippet, context    string
	label               string
	lastUnit            int
	deletionOnly        bool
	snippetLines        int
	firstLine, lastLine int
}

// selectionRange returns the first and last selected unit; without a range
// selection it is the cursor unit.
func (m *Model) selectionRange() (lo, hi int) {
	fv := m.fv
	lo, hi = fv.cursor, fv.cursor
	if fv.sel >= 0 {
		lo, hi = min(fv.sel, fv.cursor), max(fv.sel, fv.cursor)
	}
	return lo, hi
}

// selectionAnchor describes the selected lines. ok is false when the
// selection holds no diff line (a hunk header or a fold only).
func (m *Model) selectionAnchor() (a anchorInfo, ok bool) {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return a, false
	}
	e := fv.e
	f := &e.file
	lo, hi := m.selectionRange()
	a.path = e.path
	a.lastUnit = hi

	var newTexts, delTexts []string
	newMin, newMax := 0, 0
	oldMin, oldMax := 0, 0
	lastDelHunk, lastDelIdx := -1, -1
	for ui := lo; ui <= hi; ui++ {
		u := fv.units[ui]
		if u.kind == uHunk || u.kind == uFold {
			continue
		}
		l, r := u.lines(f)
		if r != nil {
			newTexts = append(newTexts, r.Text)
			if newMin == 0 || r.NewNo < newMin {
				newMin = r.NewNo
			}
			newMax = max(newMax, r.NewNo)
		}
		if l != nil && l.Kind == diff.Del {
			delTexts = append(delTexts, l.Text)
			if oldMin == 0 || l.OldNo < oldMin {
				oldMin = l.OldNo
			}
			oldMax = max(oldMax, l.OldNo)
			lastDelHunk, lastDelIdx = int(u.hunk), int(u.l)
		}
	}
	switch {
	case newMin > 0:
		a.newStart, a.newEnd = newMin, newMax
		a.oldStart, a.oldEnd = oldMin, oldMax
		if content, have := m.freshContent(e); have {
			an := diff.NewAnchor(e.path, newMin, newMax, content)
			a.snippet = an.Snippet
			a.context = joinContext(an)
		} else {
			a.snippet = strings.Join(newTexts, "\n")
			a.context = a.snippet
		}
		a.label = m.rangeLabel(newMin, newMax)
	case oldMin > 0:
		a.deletionOnly = true
		a.oldStart, a.oldEnd = oldMin, oldMax
		a.snippet = strings.Join(delTexts, "\n")
		a.context = a.snippet
		a.newStart = m.positionAfterDeletion(lastDelHunk, lastDelIdx)
		a.newEnd = a.newStart
		a.label = m.rangeLabel(oldMin, oldMax) + " removed"
	default:
		return a, false
	}
	return a, true
}

// joinContext returns the lines before, of and after an anchor.
func joinContext(an diff.Anchor) string {
	lines := make([]string, 0, len(an.Before)+len(an.After)+1)
	lines = append(lines, an.Before...)
	lines = append(lines, an.Snippet)
	lines = append(lines, an.After...)
	return strings.Join(lines, "\n")
}

// positionAfterDeletion returns the new-side line where deleted lines used
// to be: the first following line that exists on the new side, else the end
// of the hunk.
func (m *Model) positionAfterDeletion(hunk, idx int) int {
	h := &m.fv.e.file.Hunks[hunk]
	for i := idx + 1; i < len(h.Lines); i++ {
		if n := h.Lines[i].NewNo; n > 0 {
			return n
		}
	}
	return max(h.NewStart+h.NewLines, 1)
}
