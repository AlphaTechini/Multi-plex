package app

import (
	"sort"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// paletteOverlay is the command palette: a fuzzy filter over every action.
type paletteOverlay struct {
	query   lineEdit
	all     []action
	results []paletteResult
	sel     int
	scroll  int
}

// paletteResult is an action that matched, with the matched rune positions.
type paletteResult struct {
	act     action
	matched []int
	score   int
}

func newPalette(m *Model) *paletteOverlay {
	p := &paletteOverlay{all: m.paletteActions()}
	p.filter()
	return p
}

// filter recomputes the results for the query. Results keep the groups in
// their listing order; within a group the best match comes first.
func (p *paletteOverlay) filter() {
	q := strings.ToLower(strings.ReplaceAll(p.query.String(), " ", ""))
	p.results = p.results[:0]
	for _, a := range p.all {
		if q == "" {
			p.results = append(p.results, paletteResult{act: a})
			continue
		}
		if score, pos, ok := fuzzyMatch(a.title, q); ok {
			p.results = append(p.results, paletteResult{act: a, matched: pos, score: score})
		}
	}
	if q != "" {
		order := map[string]int{}
		for _, r := range p.results {
			if _, ok := order[r.act.group]; !ok {
				order[r.act.group] = len(order)
			}
		}
		sort.SliceStable(p.results, func(i, j int) bool {
			a, b := p.results[i], p.results[j]
			if a.act.group != b.act.group {
				return order[a.act.group] < order[b.act.group]
			}
			return a.score > b.score
		})
	}
	p.sel = min(p.sel, max(len(p.results)-1, 0))
	p.scroll = 0
}

// fuzzyMatch reports whether the lower-case query q is a subsequence of
// title, with a score that prefers consecutive and word-start matches.
func fuzzyMatch(title, q string) (score int, pos []int, ok bool) {
	rs := []rune(title)
	qi := 0
	prev := -2
	for i, r := range rs {
		if qi >= len(q) {
			break
		}
		if unicode.ToLower(r) != rune(q[qi]) {
			continue
		}
		pos = append(pos, i)
		switch {
		case i == prev+1:
			score += 6
		case i == 0 || !unicode.IsLetter(rs[i-1]):
			score += 4
		default:
			score++
		}
		prev = i
		qi++
	}
	if qi < len(q) {
		return 0, nil, false
	}
	return score - len(rs)/8, pos, true
}

func (p *paletteOverlay) update(m *Model, msg tea.Msg) (overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return nil, nil
		case "enter":
			if p.sel < len(p.results) {
				act := p.results[p.sel].act
				// Close first: the action may open another overlay.
				m.popOverlay()
				cmd := act.run(m)
				// popOverlay removed us; tell overlayInput we are gone.
				return nil, cmd
			}
			return nil, nil
		case "up", "ctrl+p", "shift+tab":
			p.move(-1)
		case "down", "ctrl+n", "tab":
			p.move(1)
		case "pgup":
			p.move(-8)
		case "pgdown":
			p.move(8)
		default:
			if p.query.Key(msg) {
				p.filter()
			}
		}
	case tea.PasteMsg:
		p.query.Insert(msg.Content)
		p.filter()
	case tea.MouseClickMsg:
		return p, nil
	}
	return p, nil
}

func (p *paletteOverlay) move(d int) {
	if len(p.results) == 0 {
		return
	}
	p.sel = (p.sel + d + len(p.results)) % len(p.results)
}

func (p *paletteOverlay) view(m *Model, screen string) string {
	t := m.th
	mo := kit.ModalOptions{Width: 78, Flush: true, Screen: screen, ScreenWidth: m.w, ScreenHeight: m.h, Top: max(m.h/7, 2)}
	w := kit.ModalInnerWidth(mo)

	var lines []string
	cursor := func(s string) string { return paintBg(t.Text.Strong, t.Accent.Base, false, s) }
	plain := func(s string) string { return paint(t.Text.Strong, false, s) }
	input := " " + paint(t.Accent.Base, true, t.Glyphs.ChevronRight) + " " + p.query.Render(w-14, plain, cursor)
	esc := kit.KeyHint(t, kit.Hint{Keys: "esc", Label: "close"}) + " "
	lines = append(lines, "", kit.PadRight(input, w-kit.Width(esc))+esc, "")
	lines = append(lines, kit.Divider(t, kit.DividerOptions{Width: w}))

	// Build the visible rows: group headers and results, windowed around the
	// selection.
	type row struct {
		text   string
		result int // index into results, -1 for headers and gaps
	}
	var rows []row
	group := ""
	for i, r := range p.results {
		if r.act.group != group {
			rows = append(rows, row{"", -1}, row{kit.Section(t, kit.SectionOptions{Width: w, Label: r.act.group}), -1})
			group = r.act.group
		}
		right := ""
		if sc := r.act.shortcut(m.prefix); sc != "" {
			right = paint(t.Text.Muted, false, kit.FormatKeys(t.Glyphs, sc))
		}
		rows = append(rows, row{kit.Row(t, kit.RowOptions{
			Width: w, Left: " " + p.highlight(m, r, i == p.sel), Right: right + " ",
			Selected: i == p.sel, Focused: true, Bg: t.Surface.Overlay,
		}), i})
	}
	if len(p.results) == 0 {
		rows = append(rows, row{"", -1}, row{kit.Fit(" "+paint(t.Text.Faint, false, "No matching command"), w), -1})
	}
	room := max(m.h-mo.Top-9, 4)
	selRow := 0
	for i, r := range rows {
		if r.result == p.sel {
			selRow = i
		}
	}
	if selRow < p.scroll {
		p.scroll = max(selRow-1, 0) // keep the group header in view when moving up
	}
	if selRow >= p.scroll+room {
		p.scroll = selRow - room + 1
	}
	p.scroll = min(max(p.scroll, 0), max(len(rows)-room, 0))
	for i := p.scroll; i < min(len(rows), p.scroll+room); i++ {
		lines = append(lines, rows[i].text)
	}
	lines = append(lines, "", kit.Divider(t, kit.DividerOptions{Width: w}))
	lines = append(lines, kit.Fit(" "+kit.KeyHints(t, []kit.Hint{
		{Keys: "up down", Label: "move"}, {Keys: "enter", Label: "run"},
	}, 0), w)) // esc is shown at the top right
	mo.Body = strings.Join(lines, "\n")
	return kit.Modal(t, mo)
}

// highlight paints the matched characters of a result in accent bold.
func (p *paletteOverlay) highlight(m *Model, r paletteResult, selected bool) string {
	t := m.th
	base := t.Text.Primary
	if selected {
		base = t.Text.Strong
	}
	if len(r.matched) == 0 {
		return paint(base, false, r.act.title)
	}
	at := map[int]bool{}
	for _, i := range r.matched {
		at[i] = true
	}
	var b strings.Builder
	for i, ch := range []rune(r.act.title) {
		if at[i] {
			b.WriteString(paint(t.Accent.Base, true, string(ch)))
		} else {
			b.WriteString(paint(base, false, string(ch)))
		}
	}
	return b.String()
}
