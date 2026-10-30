package specview

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// table draws a markdown table: header strong, a rule under it, columns
// separated by two cells. Cells are plain text, cut to fit the width.
func (r *renderer) table(b block) {
	t := r.t
	var rows [][]string
	for _, l := range b.lines {
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(l, "|")
		l = strings.TrimSuffix(l, "|")
		cells := strings.Split(l, "|")
		for i := range cells {
			cells[i] = plainText(t, strings.TrimSpace(cells[i]))
		}
		rows = append(rows, cells)
	}
	cols := 0
	for _, row := range rows {
		cols = max(cols, len(row))
	}
	if cols == 0 {
		return
	}
	widths := make([]int, cols)
	for _, row := range rows {
		for i, c := range row {
			widths[i] = max(widths[i], ansi.StringWidth(c))
		}
	}
	// Shrink the widest columns until the table fits.
	for total(widths)+2*(cols-1) > r.width {
		wi := 0
		for i, w := range widths {
			if w > widths[wi] {
				wi = i
			}
		}
		if widths[wi] <= 4 {
			break
		}
		widths[wi]--
	}
	r.blank()
	for ri, row := range rows {
		var sb strings.Builder
		for i := range cols {
			c := ""
			if i < len(row) {
				c = row[i]
			}
			c = ansi.Truncate(c, widths[i], t.Glyphs.Ellipsis)
			pad := strings.Repeat(" ", max(widths[i]-ansi.StringWidth(c), 0))
			if ri == 0 {
				sb.WriteString(paint(t.Text.Strong, true, c))
			} else {
				sb.WriteString(paint(t.Text.Primary, false, c))
			}
			sb.WriteString(pad)
			if i < cols-1 {
				sb.WriteString("  ")
			}
		}
		r.add(sb.String())
		if ri == 0 {
			r.add(paint(t.Border.Default, false, strings.Repeat(t.Glyphs.Rule, min(total(widths)+2*(cols-1), r.width))))
		}
	}
	r.add("")
}

func total(ws []int) int {
	n := 0
	for _, w := range ws {
		n += w
	}
	return n
}
