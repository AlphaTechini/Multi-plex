package syntax

import (
	"strings"
	"sync/atomic"

	"github.com/alphatechini/multi-plex/internal/highlight"
)

const (
	// chunkLines is how many lines one background job tokenises.
	chunkLines = 128
	// tailLines is how many lines after a chunk are tokenised with it (and
	// dropped) so a construct that ends shortly after the chunk is closed.
	tailLines = 64
	// smallFile is the size up to which one job highlights the whole file.
	smallFile = 600
	// fullLimit is the largest file that also gets an exact whole-file pass in
	// the background after its first chunks arrive.
	fullLimit = 8000
	// leadLines is how many lines before a chunk are tokenised with it (and
	// dropped) so multi-line strings and comments that started earlier are
	// still lexed correctly.
	leadLines = 128
)

var sourceIDs atomic.Uint64

// Source is one file's text, split into lines, with cached syntax tokens
// for the chunks that have been highlighted. It is owned by one goroutine
// (the Bubble Tea update loop): background jobs never touch it, they return
// a ChunkMsg that the owner passes to Apply.
type Source struct {
	id       uint64
	hl       highlight.Highlighter
	path     string
	lines    []string
	chunks   []chunkState
	language string
	full     fullState
	prev     *Source
}

// fullState tracks the whole-file pass.
type fullState uint8

const (
	fullNone fullState = iota
	fullInflight
	fullDone
)

type chunkState struct {
	toks     [][]highlight.Token
	inflight bool
}

// NewSource splits content into lines (dropping carriage returns and the
// final line terminator) and prepares an empty highlight cache. A nil
// highlighter gives a Source that never has tokens.
func NewSource(h highlight.Highlighter, path, content string) *Source {
	s := &Source{id: sourceIDs.Add(1), hl: h, path: path}
	if content != "" {
		s.lines = strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	}
	for i, l := range s.lines {
		if strings.HasSuffix(l, "\r") {
			s.lines[i] = l[:len(l)-1]
		}
	}
	s.chunks = make([]chunkState, (len(s.lines)+chunkLines-1)/chunkLines)
	if h != nil {
		s.language = h.Language(path)
	}
	return s
}

// Path returns the file path the Source was created with.
func (s *Source) Path() string { return s.path }

// Language returns the detected language name, or "".
func (s *Source) Language() string { return s.language }

// NumLines returns the number of lines.
func (s *Source) NumLines() int { return len(s.lines) }

// Text returns line i (0-based) as plain text, or "" when out of range.
func (s *Source) Text(i int) string {
	if i < 0 || i >= len(s.lines) {
		return ""
	}
	return s.lines[i]
}

// Tokens returns line i's tokens, or nil when its chunk has not been
// highlighted yet (draw Text(i) as plain text meanwhile). A highlighted
// empty line also returns nil.
func (s *Source) Tokens(i int) []highlight.Token {
	c := i / chunkLines
	if i < 0 || c >= len(s.chunks) {
		return nil
	}
	if s.chunks[c].toks == nil {
		if p := s.prev; p != nil && i < len(p.lines) && p.lines[i] == s.lines[i] {
			return p.Tokens(i) // unchanged line of the previous version: no flicker
		}
		return nil
	}
	if j := i - c*chunkLines; j < len(s.chunks[c].toks) {
		return s.chunks[c].toks[j]
	}
	return nil
}

// Ready reports whether line i's chunk has been highlighted.
func (s *Source) Ready(i int) bool {
	c := i / chunkLines
	return i >= 0 && c < len(s.chunks) && s.chunks[c].toks != nil
}

// Inherit lets s borrow prev's tokens for lines whose text is unchanged at
// the same index until s has its own, so reloading a file after an edit does
// not flash it back to plain text. prev is not modified except that its own
// ancestry is dropped.
func (s *Source) Inherit(prev *Source) {
	if prev == nil || prev.path != s.path {
		return
	}
	prev.prev = nil
	s.prev = prev
}
