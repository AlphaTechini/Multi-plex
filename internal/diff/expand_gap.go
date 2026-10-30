package diff

import "strings"

// ExpandGap reveals the unchanged lines of a gap between hunks by inserting
// them into the model as context lines (flagged Expanded, so Rows never
// collapses them again). gap uses the numbering of Row.Gap for gaps around
// hunks: gap i lies before Hunks[i], and gap len(Hunks) after the last hunk.
//
// Expanding a gap between two hunks merges them into one hunk, so later
// gap ids shift down by one; callers re-read ids from Rows. newContent is
// the new version of the file, one entry per line without line endings (see
// SplitLines). It reports whether the model changed: false for a bad gap
// index, an empty gap, or content too short to cover it.
func ExpandGap(f *File, gap int, newContent []string) bool {
	if gap < 0 || gap > len(f.Hunks) || len(f.Hunks) == 0 {
		return false
	}
	// First line of the gap on each side, and one past its last new line.
	oldFrom, newFrom := 1, 1
	if gap > 0 {
		p := &f.Hunks[gap-1]
		oldFrom = hunkFirst(p.OldStart, p.OldLines) + p.OldLines
		newFrom = hunkFirst(p.NewStart, p.NewLines) + p.NewLines
	}
	var newTo int
	if gap < len(f.Hunks) {
		n := &f.Hunks[gap]
		newTo = hunkFirst(n.NewStart, n.NewLines)
	} else {
		newTo = len(newContent) + 1
	}
	count := newTo - newFrom
	if count <= 0 || newTo-1 > len(newContent) {
		return false
	}
	ins := make([]Line, count)
	for i := range ins {
		text := newContent[newFrom-1+i]
		l := Line{Kind: Context, OldNo: oldFrom + i, NewNo: newFrom + i, Expanded: true}
		if n := len(text); n > 0 && text[n-1] == '\r' {
			text = text[:n-1]
			l.CRLF = true
		}
		l.Text = text
		ins[i] = l
	}

	switch {
	case gap == 0:
		h := &f.Hunks[0]
		h.Lines = joinLines(ins, h.Lines)
		h.OldStart, h.NewStart = 1, 1
		h.OldLines += count
		h.NewLines += count
	case gap == len(f.Hunks):
		h := &f.Hunks[gap-1]
		h.Lines = joinLines(h.Lines, ins)
		h.OldLines += count
		h.NewLines += count
	default:
		p, n := &f.Hunks[gap-1], f.Hunks[gap]
		p.Lines = joinLines(p.Lines, ins, n.Lines)
		p.OldLines += count + n.OldLines
		p.NewLines += count + n.NewLines
		f.Hunks = append(f.Hunks[:gap], f.Hunks[gap+1:]...)
	}
	return true
}

// joinLines concatenates line slices into a fresh slice, so shared backing
// arrays (the parser's arena) are never overwritten.
func joinLines(parts ...[]Line) []Line {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]Line, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// SplitLines splits file content into lines for ExpandGap and the anchoring
// functions. "\n" and "\r\n" both end a line; a final line terminator does not
// start a new empty line. Carriage returns are kept (ExpandGap strips them).
func SplitLines(content string) []string {
	if content == "" {
		return nil
	}
	content = strings.TrimSuffix(content, "\n")
	return strings.Split(content, "\n")
}
