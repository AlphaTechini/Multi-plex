package chromahl

import (
	"strings"

	"github.com/alecthomas/chroma/v2"

	"github.com/alphatechini/multi-plex/internal/highlight"
)

// maxHighlightBytes is the largest source that is tokenised; bigger inputs
// are returned as plain lines so a huge generated file cannot stall the UI.
const maxHighlightBytes = 2 << 20

// Highlighter highlights source with chroma. It is safe for concurrent use.
type Highlighter struct {
	cache lexerCache
}

var _ highlight.Highlighter = (*Highlighter)(nil)

// New returns a Highlighter.
func New() *Highlighter { return &Highlighter{} }

// Language returns the language of path as a lower-case identifier (the
// lexer's first alias, for example "go" or "typescript"), or "" when the
// language is unknown. It is suitable as a code-fence language.
func (h *Highlighter) Language(path string) string {
	return h.cache.lookup(path).name
}

// Lines tokenises src and returns one token slice per line. Lines end at "\n";
// a "\r" before it is dropped, and a final line terminator does not start an
// extra empty line (an empty src has no lines). Adjacent tokens of the same
// kind are merged, so concatenating a line's tokens gives the line. Unknown
// languages, oversized sources and lexer failures give Plain tokens.
func (h *Highlighter) Lines(path, src string) [][]highlight.Token {
	if src == "" {
		return nil
	}
	if strings.IndexByte(src, '\r') >= 0 {
		src = strings.ReplaceAll(src, "\r\n", "\n")
	}
	e := h.cache.lookup(path)
	if e.lexer == nil || len(src) > maxHighlightBytes {
		return plainLines(src)
	}
	it, err := e.lexer.Tokenise(&chroma.TokeniseOptions{State: "root"}, src)
	if err != nil {
		return plainLines(src)
	}
	return splitLines(src, it)
}

// splitLines turns a chroma token stream into per-line neutral tokens. Token
// text is taken as sub-slices of src (no copies); if the stream does not
// reproduce src exactly, the lexer rewrote the text and the result is plain.
func splitLines(src string, it chroma.Iterator) [][]highlight.Token {
	text := strings.TrimSuffix(src, "\n")
	all := make([]highlight.Token, 0, len(text)/5+1)
	// ends[i] is the index in all where line i ends.
	ends := make([]int, 0, strings.Count(text, "\n")+1)

	off := 0      // position in text
	segStart := 0 // start of the current merged run in text
	var segKind highlight.Kind
	haveSeg := false
	flush := func(end int) {
		if haveSeg && end > segStart {
			all = append(all, highlight.Token{Text: text[segStart:end], Kind: segKind})
		}
		haveSeg = false
	}
	for tok := it(); tok != chroma.EOF; tok = it() {
		v := tok.Value
		if v == "" {
			continue
		}
		if off >= len(text) {
			// Only a lexer-added final newline may follow the text.
			if v == "\n" {
				continue
			}
			return plainLines(src)
		}
		if !strings.HasPrefix(text[off:], v) {
			// A lexer may append one newline to unterminated input.
			if len(v) == len(text)-off+1 && v[len(v)-1] == '\n' && text[off:] == v[:len(v)-1] {
				v = v[:len(v)-1]
			} else {
				return plainLines(src)
			}
		}
		kind := kindOf(tok.Type)
		for len(v) > 0 {
			nl := strings.IndexByte(v, '\n')
			part := v
			if nl >= 0 {
				part = v[:nl]
			}
			if len(part) > 0 {
				if haveSeg && segKind != kind {
					flush(off)
				}
				if !haveSeg {
					segStart, segKind, haveSeg = off, kind, true
				}
				off += len(part)
			}
			if nl < 0 {
				break
			}
			flush(off)
			ends = append(ends, len(all))
			off++ // the newline
			v = v[nl+1:]
		}
	}
	flush(off)
	if off != len(text) {
		return plainLines(src)
	}
	ends = append(ends, len(all))

	out := make([][]highlight.Token, len(ends))
	prev := 0
	for i, end := range ends {
		if end > prev {
			out[i] = all[prev:end:end]
		}
		prev = end
	}
	return out
}

// plainLines returns src as Plain single-token lines.
func plainLines(src string) [][]highlight.Token {
	text := strings.TrimSuffix(src, "\n")
	out := make([][]highlight.Token, 0, strings.Count(text, "\n")+1)
	toks := make([]highlight.Token, 0, cap(out))
	for {
		line, rest, more := strings.Cut(text, "\n")
		start := len(toks)
		if line != "" {
			toks = append(toks, highlight.Token{Text: line, Kind: highlight.Plain})
		}
		out = append(out, toks[start:len(toks):len(toks)])
		if !more {
			break
		}
		text = rest
	}
	for i := range out {
		if len(out[i]) == 0 {
			out[i] = nil
		}
	}
	return out
}
