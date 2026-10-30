package overview

import (
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

const (
	labelW     = 12 // docs/16: label column of the key/value blocks in Overview
	maxItems   = 4  // items shown per summary field before "+N more"
	itemLines  = 2  // wrapped lines shown per item
	filesWidth = 48 // widest the visible file links may be
)

// summary draws the SUMMARY section: the latest report as a key/value block,
// or the agent's last message when it has not reported yet. h picks the
// compact form (docs/16: at 110×32 the summary is compact).
func (m *model) summary(c *canvas, cw, h int) {
	t := m.th
	m.fileList, m.nFiles, m.nVisFiles = nil, 0, 0
	if !m.hasDetail {
		c.text(m.sectionLine(cw, "Summary", "", 0))
		msg, col := "Loading the report…", t.Text.Muted
		if m.loadErr != "" {
			msg, col = "Could not load the session: "+clean(m.loadErr, true), t.State.Error
		}
		c.text(kit.Fit(paint(col, false, msg), cw))
		c.gap()
		return
	}
	s := m.detail.Summary
	if s == nil {
		m.lastMessage(c, cw)
		return
	}
	meta := ""
	if m.reports > 0 {
		meta = "report " + strconv.Itoa(m.reports) + " of " + strconv.Itoa(m.reports)
	}
	c.text(m.sectionLine(cw, "Summary", meta, 0))
	compact := h < 40
	vw := cw - labelW

	check := paint(t.State.Done, false, t.Glyphs.Check) + " "
	done := cleanAll(s.Done)
	if compact && len(done) > 1 {
		for i := 1; i < len(done); i++ {
			done[i] = lowerFirst(done[i])
		}
		done = []string{strings.Join(done, "; ")}
	}
	m.kv(c, "Done", m.listRows(done, check, 2, vw, maxItems, itemLines))
	if !compact {
		m.kv(c, "Decided", m.listRows(cleanAll(s.Decisions), "", 0, vw, maxItems, itemLines))
	}
	q := paint(t.Severity.Decision, false, "?") + " "
	m.kv(c, "Open", m.listRows(cleanAll(s.OpenQuestions), q, 2, vw, maxItems, itemLines))
	if next := clean(s.Next, true); next != "" {
		m.kv(c, "Next", m.listRows([]string{next}, "", 0, vw, 1, itemLines))
	}
	m.files(c, cw, s)
	c.gap()
}

// kv adds a key/value block: the label on the first row, the rows beside it.
func (m *model) kv(c *canvas, label string, rows []string) {
	for i, r := range rows {
		key := ""
		if i == 0 {
			key = paint(m.th.Text.Muted, false, label)
		}
		c.text(kit.PadRight(key, labelW) + r)
	}
}

// listRows wraps items into display rows. Each item starts with prefix (pw
// cells wide) and wraps under it; at most maxN items and maxLines lines per
// item are shown, then a "+N more" row for the rest.
func (m *model) listRows(items []string, prefix string, pw, width, maxN, maxLines int) []string {
	t := m.th
	var rows []string
	for i, it := range items {
		if i == maxN {
			rows = append(rows, strings.Repeat(" ", pw)+paint(t.Text.Muted, false, "+"+strconv.Itoa(len(items)-maxN)+" more"))
			break
		}
		lines := wrapText(it, max(width-pw, 4))
		for j, l := range lines {
			if j == maxLines {
				rows[len(rows)-1] = kit.Truncate(rows[len(rows)-1], max(width-1, 1), "") + paint(t.Text.Primary, false, "…")
				break
			}
			lead := strings.Repeat(" ", pw)
			if j == 0 {
				lead = prefix
			}
			rows = append(rows, lead+paint(t.Text.Primary, false, l))
		}
	}
	return rows
}

// cleanAll sanitises and drops empty items.
func cleanAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = clean(s, true); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// files draws the Files row: links that open the Diff view, then "+N more".
func (m *model) files(c *canvas, cw int, s *core.Summary) {
	t := m.th
	all := cleanAll(s.FilesChanged)
	if len(all) == 0 {
		return
	}
	m.fileList = all
	avail := min(cw-labelW, 1<<30)
	vis, used := 0, 0
	for i, f := range all {
		w := kit.Width(f)
		moreW := 0
		if i+1 < len(all) {
			moreW = 2 + len("+99 more")
		}
		next := used + w
		if vis > 0 {
			next += 2
		}
		if vis > 0 && (next > filesWidth || next+moreW > avail) {
			break
		}
		used = next
		vis++
	}
	more := len(all) - vis
	m.nVisFiles = vis
	m.nFiles = vis
	if more > 0 {
		m.nFiles++
	}
	m.fileSel = min(m.fileSel, max(m.nFiles-1, 0))

	var b strings.Builder
	x := pad + labelW
	for i := 0; i < vis; i++ {
		if i > 0 {
			b.WriteString("  ")
			x += 2
		}
		f := all[i]
		w := kit.Width(f)
		if vis == 1 {
			f = kit.Truncate(f, max(avail-12, 8), "…")
			w = kit.Width(f)
		}
		if m.zone == zoneFiles && m.fileSel == i {
			b.WriteString(paintBg(t.Accent.Base, t.Surface.Selected, true, f))
		} else {
			b.WriteString(paint(t.Accent.Base, false, f))
		}
		c.hit(hit{x0: x, x1: x + w, kind: hitFile, file: i, path: all[i]})
		x += w
	}
	if more > 0 {
		b.WriteString("  ")
		x += 2
		label := "+" + strconv.Itoa(more) + " more"
		if m.zone == zoneFiles && m.fileSel == vis {
			b.WriteString(paintBg(t.Accent.Base, t.Surface.Selected, true, label))
		} else {
			b.WriteString(paint(t.Text.Muted, false, label))
		}
		c.hit(hit{x0: x, x1: x + len(label), kind: hitMoreFiles, file: vis})
	}
	c.text(kit.PadRight(paint(t.Text.Muted, false, "Files"), labelW) + b.String())
}

// lastMessage is the fallback when the agent has not called report_progress:
// its last message, with a faint note.
func (m *model) lastMessage(c *canvas, cw int) {
	t := m.th
	c.text(m.sectionLine(cw, "Summary", "", 0))
	msg := clean(m.detail.LastMessage, false)
	vw := cw - labelW
	var rows []string
	if msg == "" {
		rows = []string{paint(t.Text.Faint, false, "The agent has not said anything yet")}
	} else {
		var lines []string
		for _, para := range strings.Split(msg, "\n") {
			if strings.TrimSpace(para) == "" {
				continue
			}
			lines = append(lines, wrapText(para, vw)...)
		}
		if len(lines) > 6 {
			lines = append(lines[:5], kit.Truncate(lines[5], max(vw-1, 1), "")+"…")
		}
		for _, l := range lines {
			rows = append(rows, paint(t.Text.Secondary, false, l))
		}
	}
	rows = append(rows, paint(t.Text.Faint, false, "no structured report yet"))
	m.kv(c, "Agent said", rows)
	c.gap()
}
