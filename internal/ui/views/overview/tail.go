package overview

import (
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// approvalsFrequent is the approvals-per-hour count above which the
// approval-fatigue hint shows (docs/04).
const approvalsFrequent = 3

// tailGroups builds the groups pinned under the cards, in display order and
// ready to draw (content rows without the inset, header first). The last
// group is the first to be dropped when space is short.
func (m *model) tailGroups(cw int) [][]string {
	if !m.hasDetail {
		return nil
	}
	var groups [][]string
	if g := m.conflictGroup(cw); g != nil {
		groups = append(groups, g)
	}
	if g := m.specGroup(cw); g != nil {
		groups = append(groups, g)
	}
	if g := m.checkpointGroup(cw); g != nil {
		groups = append(groups, g)
	}
	return groups
}

// addTail draws the tail groups, each after a blank row.
func (m *model) addTail(c *canvas, groups [][]string) {
	for _, g := range groups {
		c.gap()
		for _, l := range g {
			c.text(l)
		}
	}
}

// conflictGroup lists predicted merge conflicts, one row per conflicting
// branch: "Will conflict with <session> in <file>".
func (m *model) conflictGroup(cw int) []string {
	cs := m.detail.Conflicts
	if len(cs) == 0 {
		return nil
	}
	t := m.th
	g := []string{m.sectionLine(cw, "Conflicts", "", 0)}
	checked := age(m.clock.Sub(m.checkedAt))
	if checked == "now" {
		checked = "just now"
	} else {
		checked += " ago"
	}
	const maxRows = 3
	for i, cf := range cs {
		if i == maxRows {
			g = append(g, paint(t.Text.Muted, false, "+"+strconv.Itoa(len(cs)-maxRows)+" more"))
			break
		}
		line := paint(t.Severity.Warning, false, t.Glyphs.Blocked) + " " +
			paint(t.Text.Primary, false, "Will conflict with ") +
			paint(t.Text.Strong, true, m.conflictName(cf))
		if files := cleanAll(cf.Files); len(files) > 0 {
			line += paint(t.Text.Primary, false, " in ") + paint(t.Accent.Base, false, files[0])
			if len(files) > 1 {
				line += paint(t.Text.Muted, false, " +"+strconv.Itoa(len(files)-1)+" more")
			}
		}
		line += paint(t.Text.Muted, false, "   merge-tree, checked "+checked)
		g = append(g, kit.Fit(line, cw))
	}
	return g
}

// conflictName names the other side: its session when known, else its branch
// without the "mpx/" style prefix.
func (m *model) conflictName(cf core.Conflict) string {
	if n := m.names[cf.OtherSession]; n != "" {
		return clean(n, true)
	}
	b := clean(cf.OtherBranch, true)
	if cf.OtherSession != "" {
		if i := strings.LastIndexByte(b, '/'); i >= 0 {
			b = b[i+1:]
		}
	}
	return b
}

// specGroup shows the progress of the spec this session's task belongs to.
func (m *model) specGroup(cw int) []string {
	sp := m.spec
	if sp == nil {
		return nil
	}
	t := m.th
	const nameW = 16
	pw := min(28, max(cw-nameW-12-len(strconv.Itoa(sp.CriteriaTotal))-10, 8))
	row := kit.PadRight(paint(t.Text.Primary, false, kit.Truncate(clean(sp.Slug, true), nameW-1, "…")), nameW) +
		kit.Progress(t, kit.ProgressOptions{Width: pw, Done: sp.TasksDone, Total: sp.TasksTotal, Label: true}) +
		paint(t.Text.Muted, false, "  tasks   "+noun(sp.CriteriaTotal, "criterion", "criteria"))
	return []string{m.sectionLine(cw, "Spec", "", 0), kit.Fit(row, cw)}
}

// checkpointGroup shows the restore points and the approvals of the last
// hour, with the approval-fatigue hint when they are frequent (docs/04).
func (m *model) checkpointGroup(cw int) []string {
	d := m.detail
	if d.Checkpoints == 0 && d.Approvals == 0 {
		return nil
	}
	t := m.th
	cp := paint(t.Text.Primary, false, noun(d.Checkpoints, "checkpoint", "checkpoints"))
	if d.Checkpoints == 0 {
		cp = paint(t.Text.Muted, false, "No checkpoints yet")
	}
	line := cp + paint(t.Text.Muted, false, "   "+noun(d.Approvals, "approval", "approvals")+" in the last hour")
	g := []string{m.sectionLine(cw, "Checkpoints", "", 0), kit.Fit(line, cw)}
	if d.Approvals > approvalsFrequent {
		g = append(g, kit.Fit(paint(t.Severity.Warning, false, t.Glyphs.Blocked)+" "+
			paint(t.Text.Secondary, false, "Approvals are frequent — consider making this action reversible or allow-listed"), cw))
	}
	return g
}
