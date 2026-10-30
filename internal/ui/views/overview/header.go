package overview

import (
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// header draws the task as the headline, then state, role, branch, turn and
// elapsed time ("⠹ working · implementer · ⎇ mpx/auth · turn 6 · 14m").
func (m *model) header(c *canvas, cw int) {
	t := m.th
	s := m.sess
	if m.hasDetail {
		s = &m.detail.Session
	}
	title := clean(s.Task, true)
	if title == "" {
		title = clean(s.Slug, true)
	}
	c.gap()
	c.text(kit.Fit(paint(t.Text.Strong, true, title), cw))

	sep := paint(t.Text.Faint, false, "  "+t.Glyphs.Dot+"  ")
	st := m.state()
	line := kit.StateBadge(t, st, m.frame)
	if d := clean(s.StateDetail, true); d != "" {
		col := t.Text.Muted
		if st.Loud() {
			col = st.Color(t)
		}
		line += sep + paint(col, false, d)
	}
	if r := clean(s.Role, true); r != "" {
		line += sep + paint(t.Text.Muted, false, r)
	}
	if b := clean(s.Branch, true); b != "" {
		line += sep + paint(t.Text.Muted, false, t.Glyphs.Branch+" "+b)
	}
	var tail []string
	if m.reports > 0 {
		tail = append(tail, "turn "+strconv.Itoa(m.reports))
	}
	if !s.CreatedAt.IsZero() {
		tail = append(tail, elapsed(m.clock.Sub(s.CreatedAt)))
	}
	if len(tail) > 0 {
		line += sep + paint(t.Text.Muted, false, strings.Join(tail, " "+t.Glyphs.Dot+" "))
	}
	c.text(kit.Fit(line, cw))
	c.gap()
}
