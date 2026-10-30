package diffview

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/highlight"
)

// tokenEntry is the tokens of one line and the text they were made from.
type tokenEntry struct {
	text string
	toks []highlight.Token
}

// fileTokens caches the syntax tokens of one file. Entries are keyed by line
// number per side and checked against the line's text; byText is the stale
// fallback used while a refresh re-highlights shifted lines, so colours do not
// flash to plain.
type fileTokens struct {
	newSide map[int32]tokenEntry
	oldSide map[int32]tokenEntry
	byText  map[string][]highlight.Token
}

// maxTokenLines bounds one file's token cache.
const maxTokenLines = 60000

// tokens returns the token cache of a path.
func (m *Model) tokens(path string) *fileTokens {
	ft := m.tok[path]
	if ft == nil {
		ft = &fileTokens{newSide: map[int32]tokenEntry{}, oldSide: map[int32]tokenEntry{}, byText: map[string][]highlight.Token{}}
		m.tok[path] = ft
	}
	return ft
}

// lookup returns the tokens for a diff line: exact match first, then any
// earlier tokenisation of the same text, else nil (plain).
func (ft *fileTokens) lookup(ln *diff.Line) []highlight.Token {
	if ft == nil {
		return nil
	}
	if ln.Kind == diff.Del {
		if e, ok := ft.oldSide[int32(ln.OldNo)]; ok && e.text == ln.Text {
			return e.toks
		}
	} else if e, ok := ft.newSide[int32(ln.NewNo)]; ok && e.text == ln.Text {
		return e.toks
	}
	return ft.byText[ln.Text]
}

// has reports whether the line has exact tokens.
func (ft *fileTokens) has(ln *diff.Line) bool {
	if ln.Kind == diff.Del {
		e, ok := ft.oldSide[int32(ln.OldNo)]
		return ok && e.text == ln.Text
	}
	e, ok := ft.newSide[int32(ln.NewNo)]
	return ok && e.text == ln.Text
}

// hlSeg is a run of lines of one hunk to tokenise.
type hlSeg struct {
	newSrc, oldSrc     strings.Builder
	newNos, oldNos     []int32
	newTexts, oldTexts []string
	oldWanted          []bool // old-side line is a deletion (kept in the cache)
}

// hlItem is one tokenised line coming back from a command.
type hlItem struct {
	old  bool
	no   int32
	text string
	toks []highlight.Token
}

// hlDoneMsg carries tokens for a window.
type hlDoneMsg struct {
	gen   int
	path  string
	items []hlItem
}

// highlightCmd tokenises the lines around the viewport that have no tokens
// yet. It returns nil when nothing is missing, a request is running, or the
// file's language is unknown. Tokenising runs in the command, off the loop.
func (m *Model) highlightCmd() tea.Cmd {
	fv := m.fv
	if fv == nil || m.hlBusy || len(fv.units) == 0 || m.hl == nil {
		return nil
	}
	path := fv.e.path
	if m.hl.Language(path) == "" {
		return nil
	}
	ft := m.tokens(path)
	lo, hi := m.visibleUnits()
	lo = max(lo-hlMargin, 0)
	hi = min(hi+hlMargin, len(fv.units)-1)

	// For each hunk in the window: the range of lines missing tokens.
	type span struct {
		lo, hi  int
		needOld bool
	}
	runs := map[int]*span{}
	var order []int
	for i := lo; i <= hi; i++ {
		u := fv.units[i]
		if u.kind == uHunk || u.kind == uFold {
			continue
		}
		for k, li := range [2]int32{u.l, u.r} {
			if li < 0 || (k == 1 && u.l == u.r) {
				continue
			}
			ln := &fv.e.file.Hunks[u.hunk].Lines[li]
			if ft.has(ln) {
				continue
			}
			s := runs[int(u.hunk)]
			if s == nil {
				s = &span{lo: int(li), hi: int(li)}
				runs[int(u.hunk)] = s
				order = append(order, int(u.hunk))
			}
			s.lo, s.hi = min(s.lo, int(li)), max(s.hi, int(li))
			s.needOld = s.needOld || ln.Kind == diff.Del
		}
	}
	if len(order) == 0 {
		return nil
	}
	var segs []*hlSeg
	for _, hunk := range order {
		s := runs[hunk]
		lines := fv.e.file.Hunks[hunk].Lines
		// A few lines before the first missing one give the lexer context.
		from := max(s.lo-3, 0)
		end := min(s.hi, from+600)
		seg := &hlSeg{}
		for i := from; i <= end; i++ {
			ln := &lines[i]
			if ln.Kind != diff.Del {
				seg.newSrc.WriteString(ln.Text)
				seg.newSrc.WriteByte('\n')
				seg.newNos = append(seg.newNos, int32(ln.NewNo))
				seg.newTexts = append(seg.newTexts, ln.Text)
			}
			if s.needOld && ln.Kind != diff.Add {
				seg.oldSrc.WriteString(ln.Text)
				seg.oldSrc.WriteByte('\n')
				seg.oldNos = append(seg.oldNos, int32(ln.OldNo))
				seg.oldTexts = append(seg.oldTexts, ln.Text)
				seg.oldWanted = append(seg.oldWanted, ln.Kind == diff.Del)
			}
		}
		segs = append(segs, seg)
	}
	m.hlBusy = true
	hl, gen := m.hl, m.gen
	return func() tea.Msg {
		var items []hlItem
		for _, s := range segs {
			nt := hl.Lines(path, s.newSrc.String())
			for i, no := range s.newNos {
				items = append(items, hlItem{no: no, text: s.newTexts[i], toks: tokensFor(nt, i, s.newTexts[i])})
			}
			ot := hl.Lines(path, s.oldSrc.String())
			for i, no := range s.oldNos {
				if s.oldWanted[i] {
					items = append(items, hlItem{old: true, no: no, text: s.oldTexts[i], toks: tokensFor(ot, i, s.oldTexts[i])})
				}
			}
		}
		return hlDoneMsg{gen: gen, path: path, items: items}
	}
}

// tokensFor returns line i of a tokenisation when it reproduces text, else
// one plain token (a lexer that rewrote the text must not corrupt the view).
func tokensFor(lines [][]highlight.Token, i int, text string) []highlight.Token {
	if i < len(lines) {
		n := 0
		for _, t := range lines[i] {
			n += len(t.Text)
		}
		if n == len(text) {
			var b strings.Builder
			for _, t := range lines[i] {
				b.WriteString(t.Text)
			}
			if b.String() == text {
				return lines[i]
			}
		}
	}
	return []highlight.Token{{Text: text, Kind: highlight.Plain}}
}

// onHighlight stores the tokens of a finished request.
func (m *Model) onHighlight(msg hlDoneMsg) tea.Cmd {
	m.hlBusy = false
	if msg.gen != m.gen {
		return nil
	}
	ft := m.tokens(msg.path)
	if len(ft.newSide)+len(ft.oldSide) > maxTokenLines {
		clear(ft.newSide)
		clear(ft.oldSide)
		clear(ft.byText)
	}
	for _, it := range msg.items {
		e := tokenEntry{text: it.text, toks: it.toks}
		if it.old {
			ft.oldSide[it.no] = e
		} else {
			ft.newSide[it.no] = e
		}
		ft.byText[it.text] = it.toks
	}
	if m.fv != nil && m.fv.e.path == msg.path {
		m.fv.gen++
	}
	return m.highlightCmd()
}
