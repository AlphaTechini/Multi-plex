package diff

import (
	"fmt"
	"strings"
	"testing"
)

// fileContent builds numbered lines "line N".
func fileContent(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("line %d", i+1)
	}
	return out
}

// makeDiff produces a unified diff where the lines in changed (1-based) are
// modified, using ctx lines of context, by diffing two generated files.
func makeDiff(old, new []string, ctx int) string {
	// Minimal same-length diff: changed lines are the ones that differ.
	var b strings.Builder
	b.WriteString("diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n")
	n := len(old)
	i := 0
	for i < n {
		if old[i] == new[i] {
			i++
			continue
		}
		start := max(0, i-ctx)
		end := i
		j := i
		for j < n {
			if old[j] != new[j] {
				end = j
				j++
				continue
			}
			// Look ahead: is there another change within 2*ctx?
			k := j
			for k < n && k-end <= 2*ctx && old[k] == new[k] {
				k++
			}
			if k < n && k-end <= 2*ctx {
				j = k
				continue
			}
			break
		}
		stop := min(n, end+1+ctx)
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", start+1, stop-start, start+1, stop-start)
		for l := start; l < stop; l++ {
			if old[l] == new[l] {
				b.WriteString(" " + old[l] + "\n")
			} else {
				b.WriteString("-" + old[l] + "\n+" + new[l] + "\n")
			}
		}
		// fix header counts for changed lines (each change adds one on each side already)
		i = stop
	}
	return b.String()
}

func twoVersions(n int, changed ...int) (old, new []string) {
	old = fileContent(n)
	new = append([]string(nil), old...)
	for _, c := range changed {
		new[c-1] = old[c-1] + " CHANGED"
	}
	return
}

func rowSummary(f *File, rows []Row) string {
	var parts []string
	for _, r := range rows {
		switch r.Kind {
		case RowHunkHeader:
			parts = append(parts, fmt.Sprintf("H%d", r.Hunk))
		case RowLine:
			parts = append(parts, f.Hunks[r.Hunk].Lines[r.Line].Text)
		case RowCollapsed:
			parts = append(parts, fmt.Sprintf("C%d@%d", r.Collapsed, r.Gap))
		case RowEndOfFile:
			parts = append(parts, "EOF")
		}
	}
	return strings.Join(parts, " | ")
}

func TestRowsGaps(t *testing.T) {
	old, new := twoVersions(30, 10, 20)
	files := mustParse(t, makeDiff(old, new, 3))
	f := &files[0]
	if len(f.Hunks) != 2 {
		t.Fatalf("hunks = %d", len(f.Hunks))
	}
	rows := Rows(f, RowOptions{TotalNewLines: 30})
	var collapsed []Row
	for _, r := range rows {
		if r.Kind == RowCollapsed {
			collapsed = append(collapsed, r)
		}
	}
	// Hunk0 covers 7..13, hunk1 covers 17..23: gaps 6, 3 and 7 lines.
	want := []struct{ n, gap int }{{6, 0}, {3, 1}, {7, 2}}
	if len(collapsed) != len(want) {
		t.Fatalf("rows: %s", rowSummary(f, rows))
	}
	for i, w := range want {
		if collapsed[i].Collapsed != w.n || collapsed[i].Gap != w.gap {
			t.Errorf("gap row %d = %+v, want %+v", i, collapsed[i], w)
		}
	}
	if rows[0].Kind != RowCollapsed || rows[len(rows)-1].Kind != RowEndOfFile {
		t.Errorf("row order: %s", rowSummary(f, rows))
	}
	// Total rows: 3 gaps + 2 headers + lines + EOF.
	lines := len(f.Hunks[0].Lines) + len(f.Hunks[1].Lines)
	if len(rows) != 3+2+lines+1 {
		t.Errorf("len(rows) = %d", len(rows))
	}

	// Unknown total: no trailing gap row.
	rows = Rows(f, RowOptions{})
	for _, r := range rows {
		if r.Kind == RowCollapsed && r.Gap == 2 {
			t.Errorf("trailing gap present with unknown total")
		}
	}
}

func TestRowsNoGapWhenAdjacent(t *testing.T) {
	old, new := twoVersions(10, 1, 10)
	files := mustParse(t, makeDiff(old, new, 3))
	rows := Rows(&files[0], RowOptions{TotalNewLines: 10})
	for _, r := range rows {
		if r.Kind == RowCollapsed && r.Gap != 1 {
			t.Errorf("unexpected collapsed row %+v", r)
		}
	}
}

func TestRowsEmptyFile(t *testing.T) {
	rows := Rows(&File{}, RowOptions{})
	if len(rows) != 1 || rows[0].Kind != RowEndOfFile {
		t.Errorf("rows = %+v", rows)
	}
}

func TestRowsNewAndDeletedFile(t *testing.T) {
	files := mustParse(t, "diff --git a/n b/n\nnew file mode 100644\n--- /dev/null\n+++ b/n\n@@ -0,0 +1,3 @@\n+a\n+b\n+c\n")
	rows := Rows(&files[0], RowOptions{TotalNewLines: 3})
	if len(rows) != 5 { // header, 3 lines, EOF
		t.Errorf("rows = %+v", rows)
	}
	files = mustParse(t, "diff --git a/n b/n\ndeleted file mode 100644\n--- a/n\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n")
	rows = Rows(&files[0], RowOptions{TotalNewLines: 0})
	if len(rows) != 4 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestRowsIntraHunkCollapse(t *testing.T) {
	// A diff made with 10 lines of context, shown with the default 3.
	old, new := twoVersions(40, 15, 28)
	files := mustParse(t, makeDiff(old, new, 10))
	f := &files[0]
	if len(f.Hunks) != 1 {
		t.Fatalf("hunks = %d", len(f.Hunks))
	}
	rows := Rows(f, RowOptions{Context: 3, TotalNewLines: 40})
	s := rowSummary(f, rows)
	// Leading: lines 5..14 (10 ctx) keep the last 3 -> 7 hidden.
	// Between: 16..27 (12) keep 3+3 -> 6 hidden.
	// Trailing: 29..38 keep first 3 -> 7 hidden. Then gap after hunk: 2 lines.
	for _, want := range []string{"C7@2", "C6@3", "C7@4", "C2@1"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	// Expanding a collapsed run shows its lines.
	rows2 := Rows(f, RowOptions{Context: 3, TotalNewLines: 40, Expanded: map[int]bool{3: true}})
	if len(rows2) != len(rows)-1+6 {
		t.Errorf("expanded rows %d, want %d", len(rows2), len(rows)+5)
	}
	// Collapsed row line index points at the first hidden line.
	for _, r := range rows {
		if r.Kind == RowCollapsed && r.Gap == 3 {
			if got := f.Hunks[0].Lines[r.Line].Text; got != "line 19" {
				t.Errorf("first hidden = %q", got)
			}
		}
	}
	// A larger context keeps everything.
	rows3 := Rows(f, RowOptions{Context: 10})
	for _, r := range rows3 {
		if r.Kind == RowCollapsed && r.Gap > len(f.Hunks) {
			t.Errorf("collapsed row with context 10: %+v", r)
		}
	}
}

func TestExpandGap(t *testing.T) {
	old, new := twoVersions(30, 10, 20)
	files := mustParse(t, makeDiff(old, new, 3))
	f := &files[0]

	// Middle gap merges the hunks.
	if !ExpandGap(f, 1, new) {
		t.Fatal("ExpandGap(1) = false")
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("hunks = %d", len(f.Hunks))
	}
	h := f.Hunks[0]
	if h.OldStart != 7 || h.OldLines != 17 || h.NewStart != 7 || h.NewLines != 17 || len(h.Lines) != 19 {
		t.Errorf("merged hunk = %+v (lines %d)", h, len(h.Lines))
	}
	checkNumbering(t, f, new)

	// Leading and trailing gaps.
	if !ExpandGap(f, 0, new) || !ExpandGap(f, 1, new) {
		t.Fatal("expand leading/trailing failed")
	}
	h = f.Hunks[0]
	if h.OldStart != 1 || h.NewStart != 1 || h.NewLines != 30 || h.OldLines != 30 {
		t.Errorf("full hunk = %+v", h)
	}
	checkNumbering(t, f, new)
	rows := Rows(f, RowOptions{TotalNewLines: 30})
	for _, r := range rows {
		if r.Kind == RowCollapsed {
			t.Errorf("collapsed row remains after full expansion: %+v", r)
		}
	}
	// Nothing left to expand.
	if ExpandGap(f, 0, new) || ExpandGap(f, 1, new) || ExpandGap(f, 5, new) || ExpandGap(f, -1, new) {
		t.Error("expanding an empty or invalid gap reported a change")
	}
}

// checkNumbering verifies that every line of the (single) hunk has numbers
// consistent with the new content.
func checkNumbering(t *testing.T, f *File, newContent []string) {
	t.Helper()
	oldNo, newNo := 0, 0
	for hi, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.Kind != Add {
				if oldNo != 0 && l.OldNo != oldNo+1 && hi == 0 {
					t.Errorf("old numbering jumps at %q: %d after %d", l.Text, l.OldNo, oldNo)
				}
				oldNo = l.OldNo
			}
			if l.Kind != Del {
				if l.NewNo < 1 || l.NewNo > len(newContent) {
					t.Fatalf("NewNo %d out of range", l.NewNo)
				}
				if l.Kind == Context && l.Expanded && l.Text != newContent[l.NewNo-1] {
					t.Errorf("expanded line %d = %q, want %q", l.NewNo, l.Text, newContent[l.NewNo-1])
				}
				if newNo != 0 && l.NewNo != newNo+1 && hi == 0 {
					t.Errorf("new numbering jumps at %q: %d after %d", l.Text, l.NewNo, newNo)
				}
				newNo = l.NewNo
			}
		}
	}
}

func TestExpandGapNotRecollapsed(t *testing.T) {
	old, new := twoVersions(60, 10, 50)
	files := mustParse(t, makeDiff(old, new, 3))
	f := &files[0]
	if !ExpandGap(f, 1, new) {
		t.Fatal("expand failed")
	}
	for _, r := range Rows(f, RowOptions{Context: 3}) {
		if r.Kind == RowCollapsed && r.Gap > len(f.Hunks) {
			t.Errorf("expanded lines were collapsed again: %+v", r)
		}
	}
}

func TestExpandGapShortContent(t *testing.T) {
	old, new := twoVersions(40, 5, 35)
	files := mustParse(t, makeDiff(old, new, 3))
	if ExpandGap(&files[0], 1, new[:15]) {
		t.Error("expanded with content shorter than the gap")
	}
}

func TestExpandGapDoesNotCorruptNeighbours(t *testing.T) {
	// Hunks share the parser's arena: expanding the first must leave the
	// second file's lines intact.
	raw := makeDiff(fileContent(30), func() []string { _, n := twoVersions(30, 5); return n }(), 3) +
		strings.Replace(makeDiff(fileContent(30), func() []string { _, n := twoVersions(30, 25); return n }(), 3), "f.txt", "g.txt", -1)
	files := mustParse(t, raw)
	_, new1 := twoVersions(30, 5)
	before := append([]Line(nil), files[1].Hunks[0].Lines...)
	ExpandGap(&files[0], 0, new1)
	ExpandGap(&files[0], 1, new1)
	for i, l := range files[1].Hunks[0].Lines {
		if l.Text != before[i].Text {
			t.Fatalf("second file line %d changed", i)
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a\nb\n", []string{"a", "b"}},
		{"a\nb", []string{"a", "b"}},
		{"a\n\nb\n", []string{"a", "", "b"}},
		{"\n", []string{""}},
	}
	for _, tt := range tests {
		got := SplitLines(tt.in)
		if strings.Join(got, "|") != strings.Join(tt.want, "|") || len(got) != len(tt.want) {
			t.Errorf("SplitLines(%q) = %q", tt.in, got)
		}
	}
}

func TestSideBySide(t *testing.T) {
	h := Hunk{Lines: []Line{
		{Kind: Context}, // 0
		{Kind: Del},     // 1
		{Kind: Del},     // 2
		{Kind: Del},     // 3
		{Kind: Add},     // 4
		{Kind: Context}, // 5
		{Kind: Add},     // 6
		{Kind: Add},     // 7
		{Kind: Del},     // 8
		{Kind: Add},     // 9
		{Kind: Add},     // 10
	}}
	got := SideBySide(h)
	want := []Pair{{0, 0}, {1, 4}, {2, -1}, {3, -1}, {5, 5}, {-1, 6}, {-1, 7}, {8, 9}, {-1, 10}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
	if len(SideBySide(Hunk{})) != 0 {
		t.Error("empty hunk produced pairs")
	}
}
