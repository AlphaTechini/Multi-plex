package diff

// RowKind is the kind of a display row.
type RowKind uint8

// Row kinds.
const (
	// RowHunkHeader is the "@@ -a,b +c,d @@ header" line of Hunks[Row.Hunk].
	RowHunkHeader RowKind = iota
	// RowLine is Hunks[Row.Hunk].Lines[Row.Line].
	RowLine
	// RowCollapsed stands for Row.Collapsed hidden unchanged lines.
	RowCollapsed
	// RowEndOfFile is the final row ("next file: ...").
	RowEndOfFile
)

// Row is one display row of a file. Rows are small values and refer to the
// model by index, so a view can keep the []Row of a file and render only the
// visible slice of it.
type Row struct {
	Kind RowKind
	// Hunk and Line index into File.Hunks and Hunk.Lines for RowHunkHeader
	// and RowLine. For RowCollapsed, Hunk is the hunk that follows the gap
	// (len(Hunks) for the trailing gap) and Line is the index of the first
	// hidden line for a collapsed run inside a hunk, 0 for a gap between hunks.
	Hunk, Line int
	// Collapsed is the number of hidden unchanged lines (RowCollapsed only).
	Collapsed int
	// Gap identifies a RowCollapsed region for RowOptions.Expanded and
	// ExpandGap. Ids 0..len(Hunks) are the gaps around and between hunks
	// (gap i lies before hunk i); ids above that number collapsed runs
	// inside hunks. Ids are stable while the model and options are.
	Gap int
}

// RowOptions control Rows.
type RowOptions struct {
	// Context is how many unchanged lines to keep around changes. 0 means
	// the default of 3; a negative value keeps none.
	Context int
	// Expanded holds the Gap ids of collapsed runs inside hunks that should
	// be shown in full. Gaps between hunks expand with ExpandGap, which
	// needs the file content, and so disappear from the rows.
	Expanded map[int]bool
	// TotalNewLines is the line count of the new file. 0 means unknown, and
	// the trailing gap row is omitted.
	TotalNewLines int
}

// DefaultContext is the context size used when RowOptions.Context is 0.
const DefaultContext = 3

// hunkFirst returns the first line number a range covers; git writes the
// line before the range as the start of an empty range.
func hunkFirst(start, n int) int {
	if n == 0 {
		return start + 1
	}
	return start
}

// Rows builds the display rows for one file: a collapsed row for the
// unchanged lines before the first hunk, between hunks and (when the total is
// known) after the last hunk, a header row per hunk, its lines, and a final
// end-of-file row. Runs of context lines inside a hunk that are longer than
// the context size allows are collapsed too.
func Rows(f *File, opts RowOptions) []Row {
	ctx := opts.Context
	switch {
	case ctx == 0:
		ctx = DefaultContext
	case ctx < 0:
		ctx = 0
	}
	if len(f.Hunks) == 0 {
		return []Row{{Kind: RowEndOfFile}}
	}
	total := 2
	for i := range f.Hunks {
		total += len(f.Hunks[i].Lines) + 2
	}
	rows := make([]Row, 0, total)
	nextID := len(f.Hunks) + 1

	// nextNew is the first new-side line not yet covered by a hunk.
	nextNew := 1
	for hi := range f.Hunks {
		h := &f.Hunks[hi]
		if n := hunkFirst(h.NewStart, h.NewLines) - nextNew; n > 0 {
			rows = append(rows, Row{Kind: RowCollapsed, Hunk: hi, Collapsed: n, Gap: hi})
		}
		nextNew = hunkFirst(h.NewStart, h.NewLines) + h.NewLines
		rows = append(rows, Row{Kind: RowHunkHeader, Hunk: hi})

		lines := h.Lines
		for i := 0; i < len(lines); {
			if lines[i].Kind != Context {
				rows = append(rows, Row{Kind: RowLine, Hunk: hi, Line: i})
				i++
				continue
			}
			j := i
			expanded := false
			for j < len(lines) && lines[j].Kind == Context {
				expanded = expanded || lines[j].Expanded
				j++
			}
			// Context run [i, j). Keep ctx lines next to a change: the
			// run's tail at the start of a hunk, its head at the end, both
			// in between.
			head, tail := ctx, ctx
			if i == 0 {
				head = 0
			}
			if j == len(lines) {
				tail = 0
			}
			if i == 0 && j == len(lines) {
				head = ctx
			}
			hidden := (j - i) - head - tail
			if hidden < 2 || expanded {
				for k := i; k < j; k++ {
					rows = append(rows, Row{Kind: RowLine, Hunk: hi, Line: k})
				}
				i = j
				continue
			}
			id := nextID
			nextID++
			if opts.Expanded[id] {
				for k := i; k < j; k++ {
					rows = append(rows, Row{Kind: RowLine, Hunk: hi, Line: k})
				}
				i = j
				continue
			}
			for k := i; k < i+head; k++ {
				rows = append(rows, Row{Kind: RowLine, Hunk: hi, Line: k})
			}
			rows = append(rows, Row{Kind: RowCollapsed, Hunk: hi, Line: i + head, Collapsed: hidden, Gap: id})
			for k := j - tail; k < j; k++ {
				rows = append(rows, Row{Kind: RowLine, Hunk: hi, Line: k})
			}
			i = j
		}
	}
	if opts.TotalNewLines > 0 {
		if n := opts.TotalNewLines + 1 - nextNew; n > 0 {
			rows = append(rows, Row{Kind: RowCollapsed, Hunk: len(f.Hunks), Collapsed: n, Gap: len(f.Hunks)})
		}
	}
	return append(rows, Row{Kind: RowEndOfFile, Hunk: len(f.Hunks)})
}
