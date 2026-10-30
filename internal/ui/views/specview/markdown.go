package specview

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/spec"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// docKind is which of a spec's three documents is shown.
type docKind uint8

const (
	docRequirements docKind = iota
	docDesign
	docTasks
)

// mdLine is one rendered line. bg is its surface (nil means the page).
type mdLine struct {
	text string
	bg   color.Color
}

// renderMarkdown renders markdown-lite source to lines at most width cells
// wide, in the design system's tiers: headings strong, body primary, code
// spans in the variable colour, checkboxes as the state glyphs with done
// tasks muted (docs/16 reserves strikethrough for dismissed items), and the
// requirement IDs a task satisfies as small pills. Lines carry no padding
// and no page background; the caller fits and backfills them.
func renderMarkdown(t *theme.Theme, src string, kind docKind, width int) []mdLine {
	if width < 8 {
		return nil
	}
	var reqs map[int][]string
	if kind == docTasks {
		reqs = map[int][]string{}
		for _, tk := range spec.ParseTasks(src) {
			reqs[tk.Line] = tk.Requirements
		}
	}
	r := renderer{t: t, width: width, kind: kind, reqs: reqs}
	return r.render(parseBlocks(src))
}

type renderer struct {
	t     *theme.Theme
	width int
	kind  docKind
	reqs  map[int][]string // task line -> requirement IDs
	out   []mdLine
	// critNum counts criteria under the current requirement for ID pills.
	critReq, crit int
}

// add appends a line, cut to the width as a last resort.
func (r *renderer) add(s string) { r.addBg(s, nil) }

func (r *renderer) addBg(s string, bg color.Color) {
	if ansi.StringWidth(s) > r.width {
		s = ansi.Truncate(s, r.width, r.t.Glyphs.Ellipsis)
	}
	r.out = append(r.out, mdLine{text: s, bg: bg})
}

// blank adds an empty line unless the output already ends with one.
func (r *renderer) blank() {
	if n := len(r.out); n > 0 && r.out[n-1].text != "" {
		r.add("")
	}
}

func (r *renderer) render(blocks []block) []mdLine {
	t := r.t
	for i, b := range blocks {
		switch b.kind {
		case blkBlank:
			r.blank()
		case blkHeading:
			r.heading(b, i == 0)
		case blkPara:
			r.wrapped(inline(t, b.text, t.Text.Primary, false, r.kind == docRequirements && b.reqNum > 0), "", "")
		case blkQuote:
			bar := paint(t.Text.Faint, false, t.Glyphs.BarThin+" ")
			r.wrapped(inline(t, b.text, t.Text.Secondary, false, false), bar, bar)
		case blkBullet:
			r.item(b, paint(t.Text.Muted, false, bullet(t)), 1, t.Text.Primary)
		case blkOrdered:
			r.ordered(b)
		case blkTask:
			r.task(b)
		case blkRule:
			r.blank()
			r.add(paint(t.Border.Default, false, strings.Repeat(t.Glyphs.Rule, r.width)))
			r.blank()
		case blkFence:
			r.fence(b)
		case blkTable:
			r.table(b)
		}
	}
	for len(r.out) > 0 && r.out[len(r.out)-1].text == "" {
		r.out = r.out[:len(r.out)-1]
	}
	return r.out
}

// heading draws a heading: H1 the headline (strong, bold), H2 an upper-case
// section label (muted, bold), H3 a strong bold subheading, deeper levels
// secondary bold. Every heading has a blank line before and after.
func (r *renderer) heading(b block, first bool) {
	t := r.t
	if !first {
		r.blank()
	}
	switch b.level {
	case 1:
		r.wrapped(inline(t, b.text, t.Text.Strong, true, false), "", "")
	case 2:
		r.add(paint(t.Text.Muted, true, strings.ToUpper(plainText(t, b.text))))
	case 3:
		r.wrapped(inline(t, b.text, t.Text.Strong, true, false), paint(t.Accent.Base, false, t.Glyphs.Bar+" "), paint(t.Accent.Base, false, t.Glyphs.Bar+" "))
	default:
		r.wrapped(inline(t, b.text, t.Text.Secondary, true, false), "", "")
	}
	r.add("")
}

// wrapped appends words wrapped under a first-line and a continuation prefix.
func (r *renderer) wrapped(words []word, first, rest string) {
	pw := ansi.StringWidth(first)
	for i, l := range wrapWords(words, max(r.width-pw, 1), r.t.Glyphs.Ellipsis) {
		if i == 0 {
			r.add(first + l)
		} else {
			r.add(rest + l)
		}
	}
}

// item draws a list item: prefix (pw cells) then the wrapped text, with the
// continuation lines hanging under the text.
func (r *renderer) item(b block, prefix string, pw int, fg color.Color) {
	indent := strings.Repeat("  ", min(b.level, 6))
	words := inline(r.t, b.text, fg, false, false)
	r.wrapped(words, indent+prefix+" ", indent+strings.Repeat(" ", pw+1))
}

// ordered draws a numbered item. Under a "Requirement N" heading of the
// requirements document the number becomes the criterion's ID pill (R1.2).
func (r *renderer) ordered(b block) {
	t := r.t
	if r.kind == docRequirements && b.reqNum > 0 && b.level == 0 {
		if r.critReq != b.reqNum {
			r.critReq = b.reqNum
		}
		id := "R" + strconv.Itoa(b.reqNum) + "." + b.num
		pill := pillText(t, id)
		pw := ansi.StringWidth(id) + 2
		words := inline(t, b.text, t.Text.Primary, false, true)
		r.wrapped(words, pill+" ", strings.Repeat(" ", pw+1))
		return
	}
	num := paint(t.Text.Muted, false, b.num+".")
	r.item(b, num, ansi.StringWidth(b.num)+1, t.Text.Primary)
}

// task draws a checkbox item: the state glyph, the task number muted, the
// title (muted when done), and the requirement IDs as pills at the end.
func (r *renderer) task(b block) {
	t := r.t
	glyph, gc, fg := t.Glyphs.Idle, t.Text.Muted, t.Text.Primary
	switch b.mark {
	case 'x':
		glyph, gc, fg = t.Glyphs.Check, t.State.Done, t.Text.Muted
	case '~':
		glyph, gc = t.Glyphs.Working, t.State.Working
	}
	title := b.text
	id := ""
	if m := taskNumRe.FindStringSubmatch(title); m != nil {
		id, title = m[1], m[2]
	}
	indent := strings.Repeat("  ", min(b.level, 6))
	head := indent + paint(gc, false, glyph) + " "
	hw := ansi.StringWidth(head)
	if id != "" {
		head += paint(t.Text.Muted, false, id) + "  "
		hw += ansi.StringWidth(id) + 2
	}
	words := inline(t, title, fg, false, false)
	if reqs := r.reqs[b.line]; len(reqs) > 0 {
		var pills []string
		pw := 0
		for _, q := range reqs {
			pills = append(pills, pillText(t, q))
			pw += ansi.StringWidth(q) + 3
		}
		lines := wrapWords(words, max(r.width-hw, 1), t.Glyphs.Ellipsis)
		pillStr := strings.Join(pills, " ")
		if len(lines) == 0 {
			lines = []string{""}
		}
		last := lines[len(lines)-1]
		if ansi.StringWidth(last)+2+pw-1 <= r.width-hw {
			lines[len(lines)-1] = last + "  " + pillStr
		} else {
			lines = append(lines, pillStr)
		}
		rest := strings.Repeat(" ", hw)
		for i, l := range lines {
			if i == 0 {
				r.add(head + l)
			} else {
				r.add(rest + l)
			}
		}
		return
	}
	r.wrapped(words, head, strings.Repeat(" ", hw))
}

// fence draws a code block on the sunken surface, indented one cell.
func (r *renderer) fence(b block) {
	t := r.t
	r.blank()
	for _, l := range b.lines {
		var col int
		txt := codeText(l, &col)
		r.addBg(" "+paint(t.Syntax.Plain, false, ansi.Truncate(txt, max(r.width-2, 1), t.Glyphs.Ellipsis)), t.Surface.Sunken)
	}
	if len(b.lines) == 0 {
		r.addBg("", t.Surface.Sunken)
	}
	r.add("")
}

// pillText renders a small pill: the ID on the accent tint.
func pillText(t *theme.Theme, id string) string {
	return paintBg(t.Accent.Base, t.Accent.Subtle, false, " "+id+" ")
}

// bullet is the list marker in the current glyph set.
func bullet(t *theme.Theme) string {
	if t.Glyphs.Unicode {
		return t.Glyphs.Dot
	}
	return "-"
}

// plainText strips the inline markers from s.
func plainText(t *theme.Theme, s string) string {
	var b strings.Builder
	for _, w := range inline(t, s, nil, false, false) {
		b.WriteString(ansi.Strip(w.s))
		b.WriteByte(' ')
	}
	return strings.TrimSpace(b.String())
}
