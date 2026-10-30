package overview

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// cardActions are the actions every card offers (docs/08).
var cardActions = []kit.Hint{
	{Keys: "a", Label: "ask"}, {Keys: "s", Label: "steer"},
	{Keys: "i", Label: "interrupt + steer"}, {Keys: "d", Label: "dismiss"},
}

// actionKinds lines up with cardActions.
var actionKinds = []core.NoteAction{core.NoteAsk, core.NoteSteer, core.NoteInterruptSteer, core.NoteDismiss}

// cardIndent is where card text (body, refs, actions) starts inside the card.
const cardIndent = 5

// insights draws the INSIGHTS section: as many numbered cards as fit in the
// room left under the pinned tail (conflicts, spec, checkpoints), with
// "N earlier" and "N more" rows for the cards outside the window.
func (m *model) insights(c *canvas, cw, h int) {
	t := m.th
	newCount := 0
	for _, n := range m.cards {
		if !m.seen[n.ID] {
			newCount++
		}
	}
	meta, tone := "", kit.ToneDefault
	switch {
	case newCount > 0:
		meta, tone = strconv.Itoa(newCount)+" new", kit.ToneAccent
	case len(m.cards) > 0:
		meta = noun(len(m.cards), "card", "cards")
	}
	c.text(m.sectionLine(cw, "Insights", meta, tone))

	tail := m.tailGroups(cw)
	tailRows := func() int {
		n := 0
		for _, g := range tail {
			n += len(g) + 1
		}
		return n
	}
	if len(m.cards) == 0 {
		c.text(kit.Fit(paint(t.Text.Faint, false, "No cards yet. Agents raise them with note_to_user."), cw))
		m.addTail(c, tail)
		return
	}

	opts := make([]kit.CardOptions, len(m.cards))
	heights := make([]int, len(m.cards))
	for i, n := range m.cards {
		o := kit.CardOptions{
			Width: cw, Number: i + 1, Severity: severity(n.Severity),
			Title: clean(n.Title, true), Body: clean(n.Body, false),
			Refs: cleanAll(n.Refs), Age: age(m.clock.Sub(n.At)),
			Actions:  cardActions,
			Selected: i == m.sel && m.zone == zoneCards,
			Unread:   !m.seen[n.ID],
		}
		opts[i] = o
		heights[i] = kit.CardHeight(o)
	}
	var fieldH int
	if m.in != nil {
		fieldH = kit.FieldHeight(kit.FieldOptions{Rows: 1})
		heights[m.sel] += fieldH + 1 // the box and a blank row above it
	}

	// Drop the lowest-priority tail groups until one card fits.
	for len(tail) > 0 && h-c.y()-tailRows() < heights[m.sel] {
		tail = tail[:len(tail)-1]
	}
	room := h - c.y() - tailRows()
	start, end := m.window(heights, room)
	m.top = start

	if start > 0 {
		c.hit(hit{x0: pad, x1: c.w - pad, kind: hitEarlier, card: start - 1})
		c.text(kit.Fit(paint(t.Text.Muted, false, "   "+t.Glyphs.ArrowUp+" "+noun(start, "earlier card", "earlier cards")), cw))
	}
	for i := start; i < end; i++ {
		if i > start {
			c.gap()
		}
		m.addCard(c, i, opts[i], cw)
		if m.in != nil && i == m.sel {
			m.addInput(c, cw)
		}
	}
	if more := len(m.cards) - end; more > 0 {
		c.hit(hit{x0: pad, x1: c.w - pad, kind: hitLater, card: end})
		c.text(kit.Fit(paint(t.Text.Muted, false, "   "+t.Glyphs.ArrowDown+" "+strconv.Itoa(more)+" more "+pluralCard(more)+" below"), cw))
	}
	m.addTail(c, tail)
}

func pluralCard(n int) string {
	if n == 1 {
		return "card"
	}
	return "cards"
}

// window picks the cards [start, end) shown in room rows, starting from the
// previous window so it scrolls only when the selection leaves it. The
// selected card is always inside (clipped when nothing fits).
func (m *model) window(heights []int, room int) (start, end int) {
	n := len(heights)
	start = min(max(m.top, 0), m.sel)
	for {
		budget := room
		if start > 0 {
			budget--
		}
		used := 0
		end = start
		for end < n {
			add := heights[end]
			if end > start {
				add++
			}
			reserve := 0
			if end+1 < n {
				reserve = 1
			}
			if used+add+reserve > budget {
				break
			}
			used += add
			end++
		}
		if end > m.sel || start >= m.sel {
			break
		}
		start++
	}
	if end <= start {
		end = start + 1
	}
	return start, end
}

// addCard draws card i and records its hit rectangles.
func (m *model) addCard(c *canvas, i int, o kit.CardOptions, cw int) {
	y0 := c.y()
	rows := strings.Split(kit.Card(m.th, o), "\n")
	bodyN := o.BodyLines
	if bodyN <= 0 {
		bodyN = 2
	}
	ry := y0 + 1 + bodyN
	if len(o.Refs) > 0 {
		x := pad + cardIndent
		for r, ref := range o.Refs {
			w := ansi.StringWidth(ref)
			c.hits = append(c.hits, hit{y: ry, x0: x, x1: x + w, kind: hitRef, card: i, file: r})
			x += w + 2
		}
		ry++
	}
	if len(o.Actions) > 0 {
		x := pad + cardIndent
		for k, a := range o.Actions {
			w := ansi.StringWidth(kit.FormatKeys(m.th.Glyphs, a.Keys)) + 1 + ansi.StringWidth(a.Label)
			c.hits = append(c.hits, hit{y: ry, x0: x, x1: x + w, kind: hitAction, card: i, action: actionKinds[k]})
			x += w + 3
		}
	}
	for r, row := range rows {
		c.hits = append(c.hits, hit{y: y0 + r, x0: pad, x1: c.w - pad, kind: hitCard, card: i})
		c.block(row)
	}
}

// addInput draws the ask/steer box under the selected card.
func (m *model) addInput(c *canvas, cw int) {
	t := m.th
	in := m.in
	fw := cw - 3
	in.ti.SetWidth(max(fw-4, 4))
	lines := strings.Split(kit.Field(t, kit.FieldOptions{
		Width: fw, Label: cardLabel(in.action, in.number),
		Help:    kit.KeyHints(t, []kit.Hint{{Keys: "enter", Label: "send"}, {Keys: "esc", Label: "cancel"}}, 0),
		Value:   in.ti.View(),
		Focused: true,
	}), "\n")
	lead := kit.Backfill("   ", t.Surface.Base)
	c.gap()
	for i, l := range lines {
		if i == 0 { // the label row is transparent: put it on the page surface
			l = kit.Backfill(l, t.Surface.Base)
		}
		c.block(lead + l)
	}
}

// severity maps the core severity to the kit's.
func severity(s core.Severity) kit.Severity {
	switch s {
	case core.SeverityWarning:
		return kit.SeverityWarning
	case core.SeverityDecisionNeeded:
		return kit.SeverityDecision
	default:
		return kit.SeverityInfo
	}
}
