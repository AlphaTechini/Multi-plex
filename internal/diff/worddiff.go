package diff

// Limits that keep pathological input from stalling the UI.
const (
	maxWordTokens   = 1000    // lines with more tokens are marked wholly changed
	maxEditDistance = 256     // Myers gives up beyond this many edits per pair
	minSimilarity   = 0.3     // minimum common-token ratio to pair two lines
	workBudget      = 5000000 // total Myers steps per ApplyWordDiff call
)

// ApplyWordDiff fills Line.Segments for the changed lines of every hunk in f.
//
// Within each run of Del lines immediately followed by a run of Add lines,
// the i-th deleted line is paired with the i-th added line (up to the shorter
// run) when the two are similar enough (at least 30% of their non-whitespace
// tokens in common). Both lines of a pair get segments marking the changed
// byte ranges; unpaired lines keep nil Segments, meaning "whole line". Lines
// with more than 1000 tokens get a single changed segment.
func ApplyWordDiff(f *File) {
	w := wordDiffer{budget: workBudget}
	for hi := range f.Hunks {
		lines := f.Hunks[hi].Lines
		for i := 0; i < len(lines); {
			if lines[i].Kind != Del {
				lines[i].Segments = nil
				i++
				continue
			}
			delStart := i
			for i < len(lines) && lines[i].Kind == Del {
				i++
			}
			addStart := i
			for i < len(lines) && lines[i].Kind == Add {
				i++
			}
			dels, adds := lines[delStart:addStart], lines[addStart:i]
			for k := range dels {
				dels[k].Segments = nil
			}
			for k := range adds {
				adds[k].Segments = nil
			}
			n := min(len(dels), len(adds))
			for k := 0; k < n; k++ {
				w.pair(&dels[k], &adds[k])
			}
		}
	}
}

// wordDiffer holds scratch buffers reused across line pairs.
type wordDiffer struct {
	budget  int
	ta, tb  []string // tokens
	ca, cb  []bool   // token is common to both sides
	v       []int
	trace   []int
	segTmp  []Segment
	arena   []Segment
	arenaSz int
}

// pair word-diffs one deleted/added line pair.
func (w *wordDiffer) pair(del, add *Line) {
	if del.Text == add.Text {
		w.setIdentical(del)
		w.setIdentical(add)
		return
	}
	var ok bool
	if w.ta, ok = tokenize(w.ta[:0], del.Text); !ok {
		w.wholeLine(del, add)
		return
	}
	if w.tb, ok = tokenize(w.tb[:0], add.Text); !ok {
		w.wholeLine(del, add)
		return
	}
	a, b := w.ta, w.tb
	w.ca = resetBools(w.ca, len(a))
	w.cb = resetBools(w.cb, len(b))

	// Common prefix and suffix need no search.
	lo := 0
	for lo < len(a) && lo < len(b) && a[lo] == b[lo] {
		w.ca[lo], w.cb[lo] = true, true
		lo++
	}
	ha, hb := len(a), len(b)
	for ha > lo && hb > lo && a[ha-1] == b[hb-1] {
		ha--
		hb--
		w.ca[ha], w.cb[hb] = true, true
	}
	if ha > lo && hb > lo && w.budget > 0 {
		// On failure (too different, or out of budget) nothing is marked and
		// the middle stays changed.
		w.myers(a[lo:ha], b[lo:hb], w.ca[lo:ha], w.cb[lo:hb])
	}

	if similarity(a, w.ca, b, w.cb) < minSimilarity {
		return
	}
	del.Segments = w.segments(del.Text, a, w.ca)
	add.Segments = w.segments(add.Text, b, w.cb)
}

// setIdentical gives a line whose paired counterpart has identical text (for
// example a CRLF conversion) a single unchanged segment.
func (w *wordDiffer) setIdentical(l *Line) {
	if l.Text == "" {
		return
	}
	s := w.alloc(1)
	s[0] = Segment{0, len(l.Text), false}
	l.Segments = s
}

// wholeLine marks both lines as entirely changed.
func (w *wordDiffer) wholeLine(del, add *Line) {
	for _, l := range [2]*Line{del, add} {
		if l.Text == "" {
			continue
		}
		s := w.alloc(1)
		s[0] = Segment{0, len(l.Text), true}
		l.Segments = s
	}
}

// alloc returns n segments cut from a chunked arena (one allocation per many
// lines instead of one per line).
func (w *wordDiffer) alloc(n int) []Segment {
	if len(w.arena)-w.arenaSz < n {
		w.arena = make([]Segment, max(1024, n))
		w.arenaSz = 0
	}
	s := w.arena[w.arenaSz : w.arenaSz+n : w.arenaSz+n]
	w.arenaSz += n
	return s
}

// segments converts per-token common flags to merged byte segments.
func (w *wordDiffer) segments(text string, toks []string, common []bool) []Segment {
	segs := w.segTmp[:0]
	off := 0
	for i, t := range toks {
		changed := !common[i]
		if n := len(segs); n > 0 && segs[n-1].Changed == changed {
			segs[n-1].End = off + len(t)
		} else {
			segs = append(segs, Segment{off, off + len(t), changed})
		}
		off += len(t)
	}
	// Merge changed segments separated by a single whitespace character.
	out := segs[:0]
	for i := 0; i < len(segs); i++ {
		s := segs[i]
		if !s.Changed && s.End-s.Start == 1 && isSpace(text[s.Start]) &&
			len(out) > 0 && out[len(out)-1].Changed && i+1 < len(segs) && segs[i+1].Changed {
			out[len(out)-1].End = segs[i+1].End
			i++
			continue
		}
		out = append(out, s)
	}
	w.segTmp = out[:0]
	if len(out) == 0 {
		return nil
	}
	res := w.alloc(len(out))
	copy(res, out)
	return res
}

// similarity is the share of non-whitespace tokens the two sides have in
// common: 2*common / (nonWS(a) + nonWS(b)). Whitespace-only lines count as
// fully similar.
func similarity(a []string, ca []bool, b []string, cb []bool) float64 {
	var na, nb, common int
	for i, t := range a {
		if !isSpace(t[0]) {
			na++
			if ca[i] {
				common++
			}
		}
	}
	for i, t := range b {
		if !isSpace(t[0]) {
			nb++
			if cb[i] {
				common++
			}
		}
	}
	if na+nb == 0 {
		return 1
	}
	return float64(common) / float64(na+nb)
}

func resetBools(s []bool, n int) []bool {
	if cap(s) < n {
		return make([]bool, n)
	}
	s = s[:n]
	clear(s)
	return s
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\v' || c == '\f'
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c >= 0x80
}

// isOperatorByte reports whether c can be part of a multi-character operator
// such as "==", "!=", ":=", "->" or "&&"; runs of these form one token so
// "==" to "!=" is highlighted as a whole.
func isOperatorByte(c byte) bool {
	switch c {
	case '=', '!', '<', '>', '&', '|', '+', '-', '*', '/', '%', '^', '~', ':', '?':
		return true
	}
	return false
}

// tokenize splits s into identifier runs, whitespace runs, operator runs and
// single other punctuation bytes. ok is false when s has more than maxWordTokens tokens.
// Bytes >= 0x80 count as identifier bytes so UTF-8 sequences stay whole.
func tokenize(dst []string, s string) ([]string, bool) {
	for i := 0; i < len(s); {
		j := i + 1
		switch c := s[i]; {
		case isWordByte(c):
			for j < len(s) && isWordByte(s[j]) {
				j++
			}
		case isSpace(c):
			for j < len(s) && isSpace(s[j]) {
				j++
			}
		case isOperatorByte(c):
			for j < len(s) && isOperatorByte(s[j]) {
				j++
			}
		}
		if len(dst) >= maxWordTokens {
			return dst, false
		}
		dst = append(dst, s[i:j])
		i = j
	}
	return dst, true
}

// myers marks the tokens of a and b that belong to a longest common
// subsequence, in ca and cb. It reports false (marking nothing) when the
// edit distance exceeds maxEditDistance or the work budget runs out.
func (w *wordDiffer) myers(a, b []string, ca, cb []bool) bool {
	n, m := len(a), len(b)
	maxD := min(n+m, maxEditDistance)
	off := maxD + 1
	if need := 2*maxD + 3; cap(w.v) < need {
		w.v = make([]int, need)
	}
	v := w.v[:2*maxD+3]
	clear(v)
	trace := w.trace[:0]
	found := -1
	for d := 0; d <= maxD && found < 0; d++ {
		w.budget -= d + 1
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				found = d
				break
			}
		}
		// Snapshot v[-d..d] for backtracking.
		trace = append(trace, v[off-d:off+d+1]...)
		if w.budget <= 0 && found < 0 {
			w.trace = trace[:0]
			return false
		}
	}
	w.trace = trace[:0]
	if found < 0 {
		return false
	}
	// Backtrack.
	x, y := n, m
	for d := found; d > 0; d-- {
		base := (d - 1) * (d - 1)
		prev := func(k int) int { return trace[base+k+(d-1)] }
		k := x - y
		var pk int
		if k == -d || (k != d && prev(k-1) < prev(k+1)) {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := prev(pk)
		py := px - pk
		sx, sy := px+1, py
		if pk == k+1 {
			sx, sy = px, py+1
		}
		for x > sx && y > sy {
			x--
			y--
			ca[x], cb[y] = true, true
		}
		x, y = px, py
	}
	for x > 0 && y > 0 {
		x--
		y--
		ca[x], cb[y] = true, true
	}
	return true
}
