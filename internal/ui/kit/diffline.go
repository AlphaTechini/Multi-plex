package kit

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// DiffKind is the kind of a row in the unified diff view.
type DiffKind int

// Diff row kinds.
const (
	// DiffContext is an unchanged line.
	DiffContext DiffKind = iota
	// DiffAdd is an added line.
	DiffAdd
	// DiffDel is a removed line.
	DiffDel
	// DiffHunk is a hunk header ("@@ -38,9 +38,14 @@ func …").
	DiffHunk
	// DiffFold is a collapsed run of unchanged lines.
	DiffFold
)

// DiffLineOptions describes a DiffLine.
type DiffLineOptions struct {
	// Width is the exact width in cells.
	Width int
	// Kind is the row kind.
	Kind DiffKind
	// Old and New are the line numbers on each side; 0 leaves the cell blank.
	Old, New int
	// NumberWidth is the digits per number column (default 4). Use the same
	// value for every row of a file.
	NumberWidth int
	// Spans are the code (context, add, del) or the hunk's enclosing
	// declaration (hunk).
	Spans []Span
	// Header is the hunk range for DiffHunk ("@@ -38,9 +38,14 @@").
	Header string
	// Folded is the number of hidden lines for DiffFold.
	Folded int
	// Selected marks a line in the current selection (cursor or a range
	// being commented): accent bar in the gutter, strong line numbers.
	Selected bool
	// Highlight is the new-hunk highlight, 1 when the line just arrived
	// decaying to 0 over t.Motion.HunkDecay. With reduced motion or below
	// truecolor any value above 0 shows a static accent mark instead.
	Highlight float64
}

// DiffGutterWidth returns the cells before the code column for a number
// width: marker, two number columns, sign. Comment boxes indent by it.
func DiffGutterWidth(numberWidth int) int {
	if numberWidth <= 0 {
		numberWidth = 4
	}
	return 2*numberWidth + 5
}

// DiffLine renders one row of the unified diff: a one-cell marker column, the
// old and new line numbers, the sign and the syntax-coloured code on the
// line's background, with changed words on a stronger background. Long lines
// are cut with an ellipsis; the view never wraps code.
func DiffLine(t *theme.Theme, o DiffLineOptions) string {
	w := o.Width
	if w <= 0 {
		return ""
	}
	nw := o.NumberWidth
	if nw <= 0 {
		nw = 4
	}
	switch o.Kind {
	case DiffHunk:
		return diffHunk(t, o, nw)
	case DiffFold:
		return diffFold(t, o, nw)
	}

	lineBg, gutterBg, wordBg := t.Surface.Base, t.Surface.Base, t.Surface.Base
	sign, signC := " ", t.Diff.LineNumber
	switch o.Kind {
	case DiffAdd:
		lineBg, gutterBg, wordBg = t.Diff.AddLineBg, t.Diff.AddGutterBg, t.Diff.AddWordBg
		sign, signC = t.Glyphs.Plus, t.Diff.AddSign
	case DiffDel:
		lineBg, gutterBg, wordBg = t.Diff.DelLineBg, t.Diff.DelGutterBg, t.Diff.DelWordBg
		sign, signC = t.Glyphs.Minus, t.Diff.DelSign
	}
	marker := " "
	if o.Highlight > 0 {
		if t.Animated() {
			lineBg = theme.Blend(lineBg, t.Diff.NewHunkBg, o.Highlight)
			gutterBg = theme.Blend(gutterBg, t.Diff.NewHunkBg, o.Highlight)
		} else {
			marker = fg(t.Accent.Base).paint(t.Glyphs.BarThin)
		}
	}
	numC := t.Diff.LineNumber
	if o.Selected {
		marker = fg(t.Accent.Base).paint(t.Glyphs.Bar)
		numC = t.Text.Strong
	}
	gutter := ink{fg: numC, bg: gutterBg}.paint(num(o.Old, nw)+" "+num(o.New, nw)+" ") +
		ink{fg: signC, bg: gutterBg, bold: o.Kind != DiffContext}.paint(sign) +
		ink{bg: gutterBg}.paint(" ")
	cw := w - DiffGutterWidth(nw)
	code := ""
	if cw > 0 {
		code = Backfill(Fit(renderSpans(t, o.Spans, lineBg, wordBg), cw), lineBg)
	}
	return row(Backfill(marker, t.Surface.Base)+gutter+code, w, lineBg)
}

func diffHunk(t *theme.Theme, o DiffLineOptions, nw int) string {
	bg := t.Diff.HunkBg
	marker := " "
	if o.Selected {
		marker = fg(t.Accent.Base).paint(t.Glyphs.Bar)
	}
	ctx := renderSpans(t, o.Spans, nil, nil)
	if ctx != "" {
		ctx = "  " + ctx
	}
	head := marker + spaces(DiffGutterWidth(nw)-3) + ink{fg: t.Diff.HunkFg}.paint(o.Header) + ctx
	return row(head, o.Width, bg)
}

func diffFold(t *theme.Theme, o DiffLineOptions, nw int) string {
	bg := t.Surface.Sunken
	marker := " "
	if o.Selected {
		marker = fg(t.Accent.Base).paint(t.Glyphs.Bar)
	}
	dots := strings.Repeat(t.Glyphs.Fold, 3)
	label := fmt.Sprintf("%s %d unchanged lines %s", dots, o.Folded, dots)
	if o.Folded == 1 {
		label = fmt.Sprintf("%s 1 unchanged line %s", dots, dots)
	}
	left := marker + spaces(DiffGutterWidth(nw)-1) + fg(t.Diff.FoldFg).paint(label)
	right := ""
	if o.Selected {
		right = KeyHint(t, Hint{Keys: "enter", Label: "expand"}) + " "
	}
	return row(spread(left, right, o.Width), o.Width, bg)
}

// renderSpans paints spans with their syntax colours over lineBg, with
// emphasised spans over wordBg.
func renderSpans(t *theme.Theme, spans []Span, lineBg, wordBg color.Color) string {
	var b strings.Builder
	for _, s := range spans {
		c := s.Fg
		if c == nil {
			c = t.Syntax.Plain
		}
		bg := lineBg
		if s.Emph {
			bg = wordBg
		}
		text := strings.ReplaceAll(s.Text, "\t", "    ")
		b.WriteString(ink{fg: c, bg: bg, bold: s.Bold, italic: s.Italic}.paint(text))
	}
	return b.String()
}

// num right-aligns n in w cells, blank for n <= 0.
func num(n, w int) string {
	if n <= 0 {
		return spaces(w)
	}
	return PadLeft(strconv.Itoa(n), w)
}
