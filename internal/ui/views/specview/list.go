package specview

import (
	"strconv"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// renderList draws the spec list: a section header, then each spec as two
// rows (name and tasks done/total, then its progress bar and criteria count)
// and a blank row.
func (m *model) renderList(w, h int) string {
	th := m.th
	rows := []string{m.fitLine("", w), kit.Backfill(kit.Section(th, kit.SectionOptions{Width: w, Label: "Specs", Meta: strconv.Itoa(len(m.specs))}), th.Surface.Base), m.fitLine("", w)}
	m.listY = len(rows)
	for i, s := range m.specs {
		sel := i == m.sel
		nameC := th.Text.Primary
		if sel {
			nameC = th.Text.Strong
		}
		counts := paint(th.Text.Muted, false, strconv.Itoa(s.TasksDone)+"/"+strconv.Itoa(s.TasksTotal))
		rows = append(rows, kit.Row(th, kit.RowOptions{
			Width: w, Selected: sel, Focused: m.focused,
			Left:  " " + paint(nameC, sel, kit.Truncate(clean(s.Slug, true), max(w-10, 4), "…")),
			Right: counts,
		}))
		crit := paint(th.Text.Muted, false, strconv.Itoa(s.CriteriaTotal)+" criteria")
		pw := max(w-4-cells(crit)-1, 6)
		rows = append(rows, kit.Row(th, kit.RowOptions{
			Width: w, Selected: sel, Focused: m.focused,
			Left:  " " + kit.Progress(th, kit.ProgressOptions{Width: pw, Done: s.TasksDone, Total: s.TasksTotal}),
			Right: crit,
		}))
		rows = append(rows, m.fitLine("", w))
	}
	return m.pageLines(rows, w, h)
}
