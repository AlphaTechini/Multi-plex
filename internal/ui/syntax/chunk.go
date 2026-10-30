package syntax

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/highlight"
)

// ChunkMsg is the result of one background highlighting job. Pass it to
// Source.Apply from Update; it is ignored by a Source it does not belong to.
type ChunkMsg struct {
	id    uint64
	chunk int // -1: the whole file
	toks  [][]highlight.Token
}

// Request returns a command that highlights the chunks covering lines first
// to last (0-based, inclusive) and one chunk beyond each end, skipping chunks
// that are cached or already being highlighted. It returns nil when there is
// nothing to do. Call it from Update when the visible window changes.
//
// Chunks are lexed with some context before and after them, which is exact
// except for constructs (block comments, raw strings) longer than that
// context. Files up to a few thousand lines therefore also get one exact
// whole-file pass in the background; its result replaces the chunk tokens.
// Small files get only the whole-file pass.
func (s *Source) Request(first, last int) tea.Cmd {
	if s.hl == nil || len(s.chunks) == 0 || s.full == fullDone {
		return nil
	}
	var cmds []tea.Cmd
	if s.full == fullNone && len(s.lines) <= fullLimit {
		s.full = fullInflight
		cmds = append(cmds, s.fullJob())
		if len(s.lines) <= smallFile {
			return cmds[0]
		}
	}
	if s.full == fullInflight && len(s.lines) <= smallFile {
		return nil
	}
	lo := max(first/chunkLines-1, 0)
	hi := min(last/chunkLines+1, len(s.chunks)-1)
	for c := lo; c <= hi; c++ {
		st := &s.chunks[c]
		if st.toks != nil || st.inflight {
			continue
		}
		st.inflight = true
		cmds = append(cmds, s.job(c))
	}
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

// job builds the command that tokenises chunk c.
func (s *Source) job(c int) tea.Cmd {
	start := c * chunkLines
	end := min(start+chunkLines, len(s.lines))
	from := max(start-leadLines, 0)
	window := s.lines[from:min(end+tailLines, len(s.lines))] // strings are immutable; read-only
	h, path, id := s.hl, s.path, s.id
	return func() tea.Msg {
		text := strings.Join(window, "\n") + "\n"
		all := h.Lines(path, text)
		skip := start - from
		want := end - start
		out := make([][]highlight.Token, want)
		for i := range want {
			if j := skip + i; j < len(all) {
				out[i] = all[j]
			}
		}
		return ChunkMsg{id: id, chunk: c, toks: out}
	}
}

// fullJob builds the command that tokenises the whole file.
func (s *Source) fullJob() tea.Cmd {
	lines := s.lines
	h, path, id := s.hl, s.path, s.id
	return func() tea.Msg {
		all := h.Lines(path, strings.Join(lines, "\n")+"\n")
		out := make([][]highlight.Token, len(lines))
		copy(out, all)
		return ChunkMsg{id: id, chunk: -1, toks: out}
	}
}

// Apply stores a finished job. It reports whether the message belonged to
// this Source and changed what Tokens returns (the caller should redraw).
func (s *Source) Apply(m ChunkMsg) bool {
	if m.id != s.id || m.chunk >= len(s.chunks) {
		return false
	}
	if m.chunk < 0 {
		for c := range s.chunks {
			lo := c * chunkLines
			s.chunks[c] = chunkState{toks: m.toks[lo:min(lo+chunkLines, len(m.toks))]}
		}
		s.full = fullDone
		s.prev = nil
		return true
	}
	if s.full == fullDone {
		return false // the exact pass already landed; this chunk is stale
	}
	st := &s.chunks[m.chunk]
	st.inflight = false
	if m.toks == nil {
		m.toks = [][]highlight.Token{}
	}
	st.toks = m.toks
	return true
}
