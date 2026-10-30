package codeview

import "testing"

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		ok         bool
	}{
		{"", "anything", true},
		{"sess", "src/auth/session.go", true},
		{"sgo", "src/auth/session.go", true},
		{"SESS", "src/auth/session.go", true},
		{"zz", "src/auth/session.go", false},
		{"og", "go", false},
		{"a b", "alpha beta", true},
	}
	for _, c := range cases {
		_, pos, ok := fuzzyMatch(c.pattern, c.s)
		if ok != c.ok {
			t.Errorf("fuzzyMatch(%q,%q) ok=%v want %v", c.pattern, c.s, ok, c.ok)
		}
		if ok && c.pattern != "" && len(pos) == 0 {
			t.Errorf("no positions for %q", c.pattern)
		}
	}
	a, _, _ := fuzzyMatch("sess", "src/auth/session.go")
	b, _, _ := fuzzyMatch("sess", "src/auth/other/subdir/something_else_stuff.go")
	if a <= b {
		t.Errorf("file-name match should outrank scattered match: %d <= %d", a, b)
	}
	exact, _, _ := fuzzyMatch("cache.go", "src/auth/cache.go")
	loose, _, _ := fuzzyMatch("cache.go", "src/cache/gopher_o.go")
	if exact <= loose {
		t.Errorf("exact tail should win: %d <= %d", exact, loose)
	}
}
