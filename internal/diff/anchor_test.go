package diff

import (
	"strings"
	"testing"
)

func lines(s string) []string { return strings.Split(strings.TrimSuffix(s, "\n"), "\n") }

func TestNewAnchor(t *testing.T) {
	c := lines("a\nb\nc\nd\ne\nf\ng\nh\ni\n")
	a := NewAnchor("p.go", 4, 5, c)
	if a.Snippet != "d\ne" || strings.Join(a.Before, "") != "abc" || strings.Join(a.After, "") != "fgh" {
		t.Errorf("anchor = %+v", a)
	}
	a = NewAnchor("p.go", 1, 1, c)
	if len(a.Before) != 0 || a.Snippet != "a" {
		t.Errorf("anchor = %+v", a)
	}
	a = NewAnchor("p.go", 8, 20, c)
	if a.End != 9 || a.Snippet != "h\ni" || len(a.After) != 0 {
		t.Errorf("clamped anchor = %+v", a)
	}
}

func TestRelocate(t *testing.T) {
	orig := lines(`package p

func A() int {
	x := compute()
	return x + 1
}

func B() {
	log("b")
}
`)
	anchor := NewAnchor("p.go", 4, 5, orig) // x := compute() / return x + 1

	tests := []struct {
		name      string
		content   []string
		ok        bool
		start, en int
	}{
		{"unchanged", orig, true, 4, 5},
		{"moved down", append(lines("// new\n// lines\n"), orig...), true, 6, 7},
		{"moved up", orig[2:], true, 2, 3},
		{"whitespace edit", lines("package p\n\nfunc A() int {\n    x := compute()\n    return x+1\n}\n"), true, 4, 5},
		{"line edited", lines("package p\n\nfunc A() int {\n\tx := compute2()\n\treturn x + 1\n}\n"), false, 0, 0},
		{"deleted", lines("package p\n\nfunc B() {\n\tlog(\"b\")\n}\n"), false, 0, 0},
		{"empty file", nil, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e, ok := Relocate(anchor, tt.content)
			if ok != tt.ok || s != tt.start || e != tt.en {
				t.Errorf("Relocate = %d,%d,%v want %d,%d,%v", s, e, ok, tt.start, tt.en, tt.ok)
			}
		})
	}
}

func TestRelocateDuplicates(t *testing.T) {
	orig := lines("func a() {\n\treturn nil\n}\nfunc b() {\n\treturn nil\n}\nfunc c() {\n\treturn nil\n}\n")
	// Anchor the second "return nil" (line 5).
	a := NewAnchor("p.go", 5, 5, orig)

	// Insert lines at the top: context must pick the second copy, not the
	// copy nearest the old position (line 5 is now func a's neighbour).
	moved := append(lines("// h1\n// h2\n// h3\n"), orig...)
	s, e, ok := Relocate(a, moved)
	if !ok || s != 8 || e != 8 {
		t.Errorf("duplicate relocate = %d,%d,%v want 8,8", s, e, ok)
	}
	// Remove func a entirely: b's body is now at line 2.
	s, _, ok = Relocate(a, orig[3:])
	if !ok || s != 2 {
		t.Errorf("after removing a: %d,%v want 2", s, ok)
	}
	// Context all changed: the nearest copy wins.
	bare := NewAnchor("p.go", 5, 5, orig)
	bare.Before, bare.After = nil, nil
	s, _, ok = Relocate(bare, orig)
	if !ok || s != 5 {
		t.Errorf("nearest = %d,%v want 5", s, ok)
	}
	bare.Start = 7
	s, _, _ = Relocate(bare, orig)
	if s != 8 { // lines 2, 5, 8 match; 8 is nearest to 7
		t.Errorf("nearest to 7 = %d want 8", s)
	}
}

func TestRelocateMultilineAndFuzzyContext(t *testing.T) {
	orig := lines("a\n\tb\n\tc\nd\n")
	a := NewAnchor("p.go", 2, 3, orig)
	s, e, ok := Relocate(a, lines("a\n  b\n  c\nd\n"))
	if !ok || s != 2 || e != 3 {
		t.Errorf("fuzzy = %d,%d,%v", s, e, ok)
	}
	// CRLF content still matches exactly.
	s, e, ok = Relocate(a, []string{"a\r", "\tb\r", "\tc\r", "d\r"})
	if !ok || s != 2 || e != 3 {
		t.Errorf("crlf = %d,%d,%v", s, e, ok)
	}
}

func TestChanged(t *testing.T) {
	orig := lines("one\ntwo\nthree\nfour\nfive\nsix\nseven\n")
	a := NewAnchor("p.go", 4, 4, orig) // four
	tests := []struct {
		name    string
		content []string
		want    bool
	}{
		{"same", orig, false},
		{"moved", append([]string{"x", "y"}, orig...), false},
		{"edited", lines("one\ntwo\nthree\nFOUR\nfive\nsix\nseven\n"), true},
		{"deleted", lines("one\ntwo\nthree\nfive\nsix\nseven\n"), true},
		{"whitespace only", lines("one\ntwo\nthree\n four\nfive\nsix\nseven\n"), true},
		{"edited but copy elsewhere", lines("one\ntwo\nthree\nFOUR\nfive\nsix\nseven\nfour\n"), true},
		{"context edited only", lines("one\ntwo\nTHREE\nfour\nfive\nsix\nseven\n"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Changed(a, tt.content); got != tt.want {
				t.Errorf("Changed = %v want %v", got, tt.want)
			}
		})
	}
}

func TestRelocateDegenerate(t *testing.T) {
	// Must not panic on odd anchors.
	_, _, _ = Relocate(Anchor{}, nil)
	_, _, _ = Relocate(Anchor{Start: -5, Snippet: "x"}, []string{"x"})
	_, _, _ = Relocate(Anchor{Start: 100, Snippet: "x", Before: []string{"a"}}, []string{"x"})
	_ = Changed(Anchor{}, nil)
}
