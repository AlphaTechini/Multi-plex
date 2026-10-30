package codeview

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// pickerRows is how many results the go-to-file list shows at once.
const pickerRows = 8

// pickItem is one result: a changed file, or the path typed in the box.
type pickItem struct {
	path   string
	change *vcs.FileChange // nil for a typed path
	pos    []int           // matched rune positions in path
	score  int
}

// picker is the go-to-file box: a text input over the session's changed
// files, plus whatever path was typed.
type picker struct {
	in     textinput.Model
	items  []pickItem
	sel    int
	scroll int
}

// newPicker builds an open picker listing every changed file.
func newPicker(th *theme.Theme, changes []vcs.FileChange) *picker {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 512
	st := textinput.DefaultStyles(true)
	text := st.Focused.Text.Foreground(th.Text.Strong)
	ph := st.Focused.Placeholder.Foreground(th.Text.Faint)
	st.Focused.Text, st.Blurred.Text = text, text
	st.Focused.Placeholder, st.Blurred.Placeholder = ph, ph
	st.Cursor.Color = th.Accent.Base
	st.Cursor.Blink = false
	ti.SetStyles(st)
	ti.Placeholder = "Go to a changed file, or type a path"
	ti.SetWidth(50)
	ti.Focus()
	p := &picker{in: ti}
	p.refilter(changes)
	return p
}

// refilter recomputes the result list for the current query, best match
// first (changed files keep their order when the query is empty), and keeps
// the selection on the same path when it can.
func (p *picker) refilter(changes []vcs.FileChange) {
	var keep string
	if p.sel < len(p.items) {
		keep = p.items[p.sel].path
	}
	q := strings.TrimSpace(p.in.Value())
	items := make([]pickItem, 0, len(changes)+1)
	exact := false
	for i := range changes {
		c := &changes[i]
		score, pos, ok := fuzzyMatch(q, c.Path)
		if !ok {
			continue
		}
		items = append(items, pickItem{path: c.Path, change: c, pos: pos, score: score})
		exact = exact || c.Path == normPath(q)
	}
	if q != "" {
		slices.SortStableFunc(items, func(a, b pickItem) int { return b.score - a.score })
		if !exact {
			items = append(items, pickItem{path: normPath(q)})
		}
	}
	p.items = items
	p.sel = 0
	for i, it := range items {
		if it.path == keep {
			p.sel = i
			break
		}
	}
	p.fit()
}

// move moves the selection, clamped.
func (p *picker) move(d int) {
	p.sel = min(max(p.sel+d, 0), max(len(p.items)-1, 0))
	p.fit()
}

// fit scrolls the list so the selection is visible.
func (p *picker) fit() {
	if p.sel < p.scroll {
		p.scroll = p.sel
	}
	if p.sel >= p.scroll+pickerRows {
		p.scroll = p.sel - pickerRows + 1
	}
	p.scroll = max(p.scroll, 0)
}

// selected returns the highlighted item.
func (p *picker) selected() (pickItem, bool) {
	if p.sel < 0 || p.sel >= len(p.items) {
		return pickItem{}, false
	}
	return p.items[p.sel], true
}
