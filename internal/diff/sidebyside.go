package diff

// Pair is one row of the side-by-side view. Left and Right are indices into
// Hunk.Lines, or -1 when that side is empty. A context line has Left == Right.
type Pair struct {
	Left, Right int
}

// SideBySide pairs the lines of a hunk for the side-by-side view. In each run
// of deleted lines followed by added lines, the i-th deletion sits beside the
// i-th addition; the surplus of the longer run gets an empty opposite side.
// Context lines appear on both sides.
func SideBySide(h Hunk) []Pair {
	lines := h.Lines
	pairs := make([]Pair, 0, len(lines))
	for i := 0; i < len(lines); {
		switch lines[i].Kind {
		case Context:
			pairs = append(pairs, Pair{i, i})
			i++
		case Add:
			pairs = append(pairs, Pair{-1, i})
			i++
		default:
			ds := i
			for i < len(lines) && lines[i].Kind == Del {
				i++
			}
			as := i
			for i < len(lines) && lines[i].Kind == Add {
				i++
			}
			nd, na := as-ds, i-as
			for k := 0; k < max(nd, na); k++ {
				p := Pair{-1, -1}
				if k < nd {
					p.Left = ds + k
				}
				if k < na {
					p.Right = as + k
				}
				pairs = append(pairs, p)
			}
		}
	}
	return pairs
}
