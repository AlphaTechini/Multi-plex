package specview

import (
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// docNames are the three documents' tab labels, in kind order.
var docNames = [3]string{"Requirements", "Design", "Tasks"}

// docFiles are the files they come from.
var docFiles = [3]string{"requirements.md", "design.md", "tasks.md"}

// renderDetail draws the selected spec: title, progress, the document tabs
// with the active one marked by a heavy accent rule, and the document body
// scrolled to the stored position. x0 is the pane's left edge in view
// coordinates, for tab hit boxes.
func (m *model) renderDetail(w, h, x0 int) string {
	th := m.th
	s, ok := m.current()
	if !ok {
		return m.pageLines(nil, w, h)
	}
	cw := w - 2*pad
	line := func(content string) string { return m.fitLine(strings.Repeat(" ", pad)+content, w) }
	rows := make([]string, 0, h)

	if !m.twoPane {
		rows = append(rows, line(paint(th.Text.Faint, false, th.Glyphs.ChevronLeft+" all specs")))
	} else {
		rows = append(rows, m.fitLine("", w))
	}
	title := clean(s.Title, true)
	if title == "" {
		title = clean(s.Slug, true)
	}
	rows = append(rows, line(kit.Fit(paint(th.Text.Strong, true, title), cw)))
	slug := clean(s.Slug, true)
	tail := "  tasks   " + strconv.Itoa(s.CriteriaTotal) + " criteria"
	pw := min(max(cw-cells(slug)-3-cells(tail), 8), 24)
	rows = append(rows, line(kit.Fit(paint(th.Text.Muted, false, slug)+"   "+
		kit.Progress(th, kit.ProgressOptions{Width: pw, Done: s.TasksDone, Total: s.TasksTotal, Label: true})+
		paint(th.Text.Muted, false, tail), cw)))
	rows = append(rows, m.fitLine("", w))

	// Tabs and the rule under them.
	var tabs, rule strings.Builder
	x := pad
	for k := range docNames {
		label := docNames[k]
		if docKind(k) == docTasks && s.TasksTotal > 0 {
			label += " " + strconv.Itoa(s.TasksDone) + "/" + strconv.Itoa(s.TasksTotal)
		}
		lw := cells(label)
		m.tabX[k] = [2]int{x0 + x, x0 + x + lw}
		if docKind(k) == m.kind {
			tabs.WriteString(paint(th.Text.Strong, true, docNames[k]))
			if label != docNames[k] {
				tabs.WriteString(paint(th.Text.Muted, false, label[len(docNames[k]):]))
			}
			rule.WriteString(paint(th.Accent.Base, false, strings.Repeat(th.Glyphs.RuleHeavy, lw)))
		} else {
			tabs.WriteString(paint(th.Text.Muted, false, label))
			rule.WriteString(paint(th.Border.Default, false, strings.Repeat(th.Glyphs.Rule, lw)))
		}
		if k < len(docNames)-1 {
			tabs.WriteString("   ")
			rule.WriteString(paint(th.Border.Default, false, strings.Repeat(th.Glyphs.Rule, 3)))
			x += lw + 3
		}
	}
	d := m.docs[s.Slug]
	lines, scroll, pct := m.docLines(s.Slug, d, cw, h-bodyTop)
	tabsLine := kit.Fit(tabs.String(), cw)
	if pct != "" {
		tabsLine = kit.PadRight(kit.Truncate(tabs.String(), max(cw-cells(pct)-1, 0), "…"), cw-cells(pct)) + paint(th.Text.Faint, false, pct)
	}
	rows = append(rows, line(tabsLine))
	ruleStr := rule.String()
	if fill := cw - kit.Width(ruleStr); fill > 0 {
		ruleStr += paint(th.Border.Default, false, strings.Repeat(th.Glyphs.Rule, fill))
	}
	rows = append(rows, line(ruleStr))
	rows = append(rows, m.fitLine("", w))

	// Body.
	for i := scroll; i < len(lines) && len(rows) < h; i++ {
		l := lines[i]
		if l.bg != nil {
			edge := kit.Backfill(strings.Repeat(" ", pad), th.Surface.Base)
			rows = append(rows, edge+kit.Backfill(kit.Fit(l.text, cw), l.bg)+edge)
			continue
		}
		rows = append(rows, line(l.text))
	}
	return m.pageLines(rows, w, h)
}

// docLines returns the rendered lines of the selected document, the clamped
// scroll offset, and a scroll percentage label ("" when everything fits). It
// renders on first use for a width and caches the result.
func (m *model) docLines(slug string, d *doc, cw, rows int) ([]mdLine, int, string) {
	th := m.th
	if d == nil || d.state == notLoaded || d.state == loading {
		return []mdLine{{text: paint(th.Text.Muted, false, "Loading…")}}, 0, ""
	}
	if d.state == failed {
		return []mdLine{{text: paint(th.State.Error, false, "Could not read the spec: "+d.err)}}, 0, ""
	}
	if d.width != cw {
		d.lines = [3][]mdLine{}
		d.width = cw
	}
	if d.lines[m.kind] == nil {
		src := [3]string{d.spec.Requirements, d.spec.Design, d.spec.Tasks}[m.kind]
		if strings.TrimSpace(src) == "" {
			d.lines[m.kind] = []mdLine{{text: paint(th.Text.Faint, false, emptyDoc(m.kind))}}
		} else {
			d.lines[m.kind] = renderMarkdown(th, clean(src, false), m.kind, cw)
		}
	}
	lines := d.lines[m.kind]
	scroll := min(m.scroll[m.kind][slug], max(len(lines)-rows, 0))
	m.scroll[m.kind][slug] = scroll
	m.total = len(lines)
	pct := ""
	if len(lines) > rows && rows > 0 {
		pct = strconv.Itoa(min((scroll+rows)*100/len(lines), 100)) + "%"
	}
	return lines, scroll, pct
}

// emptyDoc says why a document is empty, following the flow of docs/09.
func emptyDoc(k docKind) string {
	switch k {
	case docRequirements:
		return "requirements.md has not been written yet. A planner session drafts it from your idea."
	case docDesign:
		return "design.md has not been written yet. The planner drafts it once the requirements are approved."
	default:
		return "tasks.md has not been written yet. The planner drafts it once the design is approved."
	}
}
