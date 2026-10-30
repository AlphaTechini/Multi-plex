package diff

import (
	"strings"
	"testing"
)

// changedText returns the changed substrings of a line joined by "|".
func changedText(l Line) string {
	var parts []string
	for _, s := range l.Segments {
		if s.Changed {
			parts = append(parts, l.Text[s.Start:s.End])
		}
	}
	return strings.Join(parts, "|")
}

func checkSegments(t *testing.T, l Line) {
	t.Helper()
	if l.Segments == nil {
		return
	}
	pos := 0
	for i, s := range l.Segments {
		if s.Start != pos || s.End <= s.Start {
			t.Fatalf("segments of %q not contiguous: %+v", l.Text, l.Segments)
		}
		if i > 0 && l.Segments[i-1].Changed == s.Changed {
			t.Fatalf("segments of %q not alternating: %+v", l.Text, l.Segments)
		}
		pos = s.End
	}
	if pos != len(l.Text) {
		t.Fatalf("segments of %q do not cover it: %+v", l.Text, l.Segments)
	}
}

func hunkOf(lines ...Line) *File {
	return &File{Hunks: []Hunk{{Lines: lines}}}
}

func TestApplyWordDiffPairs(t *testing.T) {
	tests := []struct {
		name, del, add, wantDel, wantAdd string
	}{
		{"identifier", "return foo(a, b)", "return foo(a, c)", "b", "c"},
		{"insert word", "x := compute(y)", "x := compute(y, z)", "", ", z"},
		{"merge single space", "call(one two three)", "call(uno dos three)", "one two", "uno dos"},
		{"operator", "if a == b {", "if a != b {", "==", "!="},
		{"whitespace only", "\tfoo()", "    foo()", "\t", "    "},
		{"suffix", "name := \"abc\"", "name := \"abcd\"", "abc", "abcd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := hunkOf(Line{Kind: Del, Text: tt.del}, Line{Kind: Add, Text: tt.add})
			ApplyWordDiff(f)
			ls := f.Hunks[0].Lines
			checkSegments(t, ls[0])
			checkSegments(t, ls[1])
			if got := changedText(ls[0]); got != tt.wantDel {
				t.Errorf("del changed = %q, want %q", got, tt.wantDel)
			}
			if got := changedText(ls[1]); got != tt.wantAdd {
				t.Errorf("add changed = %q, want %q", got, tt.wantAdd)
			}
		})
	}
}

func TestApplyWordDiffDissimilarAndUnpaired(t *testing.T) {
	f := hunkOf(
		Line{Kind: Context, Text: "ctx"},
		Line{Kind: Del, Text: "alpha beta gamma"},
		Line{Kind: Del, Text: "second removed line"},
		Line{Kind: Add, Text: "completely different"},
		Line{Kind: Context, Text: "end"},
	)
	ApplyWordDiff(f)
	for i, l := range f.Hunks[0].Lines {
		if l.Segments != nil {
			t.Errorf("line %d has segments %+v", i, l.Segments)
		}
	}
}

func TestApplyWordDiffUnevenRuns(t *testing.T) {
	f := hunkOf(
		Line{Kind: Del, Text: "one(1)"},
		Line{Kind: Del, Text: "two(2)"},
		Line{Kind: Del, Text: "three(3)"},
		Line{Kind: Add, Text: "one(10)"},
		Line{Kind: Add, Text: "two(20)"},
	)
	ApplyWordDiff(f)
	ls := f.Hunks[0].Lines
	if changedText(ls[0]) != "1" || changedText(ls[3]) != "10" || changedText(ls[1]) != "2" || changedText(ls[4]) != "20" {
		t.Errorf("pairs wrong: %+v", ls)
	}
	if ls[2].Segments != nil {
		t.Errorf("unpaired line has segments")
	}
}

func TestApplyWordDiffLongLine(t *testing.T) {
	a := strings.Repeat("a ", 800)
	b := strings.Repeat("a ", 799) + "b "
	f := hunkOf(Line{Kind: Del, Text: a}, Line{Kind: Add, Text: b})
	ApplyWordDiff(f)
	for _, l := range f.Hunks[0].Lines {
		if len(l.Segments) != 1 || !l.Segments[0].Changed || l.Segments[0].End != len(l.Text) {
			t.Errorf("long line not wholly changed: %+v", l.Segments)
		}
	}
}

func TestApplyWordDiffIdenticalText(t *testing.T) {
	f := hunkOf(Line{Kind: Del, Text: "same", CRLF: true}, Line{Kind: Add, Text: "same"})
	ApplyWordDiff(f)
	for _, l := range f.Hunks[0].Lines {
		checkSegments(t, l)
		if changedText(l) != "" || len(l.Segments) != 1 {
			t.Errorf("segments = %+v", l.Segments)
		}
	}
}

func TestApplyWordDiffUnicode(t *testing.T) {
	f := hunkOf(Line{Kind: Del, Text: "msg := \"héllo wörld\""}, Line{Kind: Add, Text: "msg := \"héllo wörld!\""})
	ApplyWordDiff(f)
	for _, l := range f.Hunks[0].Lines {
		checkSegments(t, l)
	}
}

func TestApplyWordDiffIdempotentAndPathological(t *testing.T) {
	// Highly different long-ish lines must finish quickly and stay valid.
	var del, add Line
	del.Kind, add.Kind = Del, Add
	var d, a strings.Builder
	for i := 0; i < 400; i++ {
		d.WriteString("x")
		d.WriteString(string(rune('a' + i%26)))
		d.WriteString(" ")
		a.WriteString("y")
		a.WriteString(string(rune('a' + (i*7)%26)))
		a.WriteString(" ")
	}
	del.Text, add.Text = d.String(), a.String()
	f := hunkOf(del, add)
	ApplyWordDiff(f)
	ApplyWordDiff(f)
	for _, l := range f.Hunks[0].Lines {
		checkSegments(t, l)
	}
}

// FuzzMyers cross-checks the common-token marks against a plain LCS.
func TestMyersMatchesLCS(t *testing.T) {
	cases := [][2]string{
		{"a b c d e f", "a c d x f"},
		{"one two three", "three two one"},
		{"", "abc"},
		{"abc abc abc", "abc"},
		{"f(a,b,c)", "f(a,c)"},
	}
	var w wordDiffer
	w.budget = 1 << 30
	for _, c := range cases {
		a, _ := tokenize(nil, c[0])
		b, _ := tokenize(nil, c[1])
		ca := make([]bool, len(a))
		cb := make([]bool, len(b))
		if !w.myers(a, b, ca, cb) {
			t.Fatalf("myers failed on %q", c)
		}
		n := 0
		for _, x := range ca {
			if x {
				n++
			}
		}
		m := 0
		for _, x := range cb {
			if x {
				m++
			}
		}
		if want := lcsLen(a, b); n != want || m != want {
			t.Errorf("%q: marked %d/%d, LCS %d", c, n, m, want)
		}
		// Marked tokens must match in order.
		var ma, mb []string
		for i, x := range ca {
			if x {
				ma = append(ma, a[i])
			}
		}
		for i, x := range cb {
			if x {
				mb = append(mb, b[i])
			}
		}
		if strings.Join(ma, "\x00") != strings.Join(mb, "\x00") {
			t.Errorf("%q: marked sequences differ: %q vs %q", c, ma, mb)
		}
	}
}

func lcsLen(a, b []string) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	return dp[len(a)][len(b)]
}

func TestTokenize(t *testing.T) {
	got, ok := tokenize(nil, "foo_1 += (bar)\t é")
	want := []string{"foo_1", " ", "+=", " ", "(", "bar", ")", "\t ", "é"}
	if !ok || strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("tokens = %q", got)
	}
}
