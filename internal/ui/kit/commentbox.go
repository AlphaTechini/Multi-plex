package kit

import (
	"image/color"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// CommentBoxOptions describes a CommentBox.
type CommentBoxOptions struct {
	// Width is the exact width in cells, indent included.
	Width int
	// Indent is the blank lead-in, normally DiffGutterWidth so the box lines
	// up with the code column.
	Indent int
	// Author is "You" or the agent name.
	Author string
	// Status is "draft", "queued", "sent" or "outdated".
	Status string
	// Range is the anchored line range ("L39–L41").
	Range string
	// Body is the comment text, wrapped.
	Body string
	// Hints are the box's actions, shown on its last row when focused.
	Hints []Hint
	// Focused is the comment being edited: accent bar and a cursor.
	Focused bool
}

// CommentBox renders an inline review comment under the lines it refers to: a
// raised block with a thin bar on its left, a header with author, status and
// line range, the wrapped body and, while editing, its key hints.
func CommentBox(t *theme.Theme, o CommentBoxOptions) string {
	w := o.Width
	ind := min(max(o.Indent, 0), max(w-4, 0))
	bw := w - ind
	if w <= 0 {
		return ""
	}
	barC := t.Text.Faint
	if o.Focused {
		barC = t.Accent.Base
	}
	lead := Backfill(spaces(ind), t.Surface.Base)
	bar := fg(barC).paint(t.Glyphs.BarThin) + " "
	cw := max(bw-3, 0)
	var lines []string
	emit := func(s string) {
		lines = append(lines, lead+row(bar+s, bw, t.Surface.Raised))
	}
	head := ink{fg: t.Text.Strong, bold: true}.paint(o.Author)
	if o.Status != "" {
		head += fg(t.Text.Faint).paint(" "+t.Glyphs.Dot+" ") + fg(commentStatusColor(t, o.Status)).paint(o.Status)
	}
	emit(spread(head, fg(t.Text.Muted).paint(o.Range), cw))
	body := wrap(o.Body, cw-1)
	for i, l := range body {
		s := fg(t.Text.Primary).paint(l)
		if o.Focused && i == len(body)-1 {
			s += cursorCell(t)
		}
		emit(s)
	}
	if o.Focused && len(o.Hints) > 0 {
		emit(KeyHints(t, o.Hints, cw))
	}
	return strings.Join(lines, "\n")
}

func commentStatusColor(t *theme.Theme, status string) color.Color {
	switch status {
	case "queued":
		return t.Accent.Base
	case "sent":
		return t.State.Done
	case "outdated":
		return t.Severity.Warning
	default:
		return t.Text.Muted
	}
}
