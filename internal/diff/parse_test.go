package diff

import (
	"os"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/vcs"
)

func mustParse(t *testing.T, raw string) []File {
	t.Helper()
	files, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return files
}

func TestParseSample(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatal(err)
	}
	files := mustParse(t, string(raw))
	byPath := map[string]*File{}
	for i := range files {
		byPath[files[i].Path()] = &files[i]
	}
	if len(files) != 9 {
		t.Fatalf("got %d files, want 9", len(files))
	}

	if f := byPath["added.txt"]; f.Status != vcs.Added || f.OldPath != "" || f.Added != 2 || f.Deleted != 0 {
		t.Errorf("added.txt = %+v", f)
	}
	if f := byPath["bin.dat"]; !f.Binary || len(f.Hunks) != 0 || f.Status != vcs.Modified {
		t.Errorf("bin.dat = %+v", f)
	}
	f := byPath["crlf.txt"]
	if len(f.Hunks) != 1 || f.Added != 1 || f.Deleted != 1 {
		t.Fatalf("crlf.txt = %+v", f)
	}
	for _, l := range f.Hunks[0].Lines {
		if strings.Contains(l.Text, "\r") {
			t.Errorf("Text contains CR: %q", l.Text)
		}
		if !l.CRLF && l.Text != "one" {
			t.Errorf("line %q lost CRLF flag", l.Text)
		}
	}
	if f := byPath["empty.txt"]; f.Status != vcs.Added || len(f.Hunks) != 0 || f.NewPath != "empty.txt" {
		t.Errorf("empty.txt = %+v", f)
	}
	if f := byPath["keep.txt"]; f.OldMode != "100644" || f.NewMode != "100755" || len(f.Hunks) != 1 || f.ModeOnly() {
		t.Errorf("keep.txt = %+v", f)
	}
	if f := byPath["new.go"]; f.Status != vcs.Renamed || f.OldPath != "old.go" || f.Similarity != 100 || len(f.Hunks) != 0 {
		t.Errorf("new.go = %+v", f)
	}
	f = byPath["nonl.txt"]
	ls := f.Hunks[0].Lines
	if len(ls) != 2 || !ls[0].NoNewlineAtEOF || !ls[1].NoNewlineAtEOF || ls[0].Text != "tail" || ls[1].Text != "tail2" {
		t.Errorf("nonl.txt lines = %+v", ls)
	}
	if f := byPath["sp ace.txt"]; f == nil || f.OldPath != "sp ace.txt" || f.Hunks[0].Lines[1].Text != "hello world" {
		t.Errorf("sp ace.txt = %+v", f)
	}
	if f := byPath["spécial.go"]; f == nil || f.OldPath != "spécial.go" || f.Added != 1 {
		t.Errorf("spécial.go = %+v (keys %v)", f, keys(byPath))
	}
}

func keys(m map[string]*File) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestParseLineNumbersAndHeader(t *testing.T) {
	raw := "diff --git a/f.go b/f.go\nindex 1..2 100644\n--- a/f.go\n+++ b/f.go\n" +
		"@@ -10,4 +10,5 @@ func Foo() {\n ctx1\n-old\n+new1\n+new2\n ctx2\n" +
		"@@ -30 +31,0 @@\n-gone\n"
	files := mustParse(t, raw)
	h := files[0].Hunks
	if len(h) != 2 {
		t.Fatalf("hunks = %d", len(h))
	}
	if h[0].Header != "func Foo() {" || h[0].OldStart != 10 || h[0].OldLines != 4 || h[0].NewLines != 5 {
		t.Errorf("hunk0 = %+v", h[0])
	}
	type no struct{ o, n int }
	want := []no{{10, 10}, {11, 0}, {0, 11}, {0, 12}, {12, 13}}
	for i, l := range h[0].Lines {
		if (no{l.OldNo, l.NewNo}) != want[i] {
			t.Errorf("line %d (%q) = %d,%d want %v", i, l.Text, l.OldNo, l.NewNo, want[i])
		}
	}
	if h[1].OldLines != 1 || h[1].NewLines != 0 || h[1].Lines[0].OldNo != 30 || h[1].Lines[0].Kind != Del {
		t.Errorf("hunk1 = %+v", h[1])
	}
	if files[0].Added != 2 || files[0].Deleted != 2 {
		t.Errorf("counts = %d/%d", files[0].Added, files[0].Deleted)
	}
}

func TestParseContentLooksLikeHeaders(t *testing.T) {
	raw := "diff --git a/a.md b/a.md\n--- a/a.md\n+++ b/a.md\n@@ -1,3 +1,3 @@\n" +
		" keep\n--- not a header\n+++ also not\n" +
		"diff --git a/b.md b/b.md\n--- a/b.md\n+++ b/b.md\n@@ -1 +1 @@\n-x\n+y\n"
	files := mustParse(t, raw)
	if len(files) != 2 {
		t.Fatalf("files = %d", len(files))
	}
	if got := files[0].Hunks[0].Lines; len(got) != 3 || got[1].Text != "-- not a header" || got[2].Text != "++ also not" {
		t.Errorf("lines = %+v", got)
	}
}

func TestParseQuotedAndSpacePaths(t *testing.T) {
	tests := []struct {
		name, raw, old, new string
		status              vcs.FileStatus
	}{
		{"quoted", "diff --git \"a/sp\\303\\251cial.go\" \"b/sp\\303\\251cial.go\"\nnew file mode 100644\n", "", "spécial.go", vcs.Added},
		{"escapes", "diff --git \"a/t\\tab\\\"q\\\\.txt\" \"b/t\\tab\\\"q\\\\.txt\"\ndeleted file mode 100644\n", "t\tab\"q\\.txt", "", vcs.Deleted},
		{"spaces", "diff --git a/my dir/a b.txt b/my dir/a b.txt\nnew file mode 100644\n", "", "my dir/a b.txt", vcs.Added},
		{"rename spaces", "diff --git a/old name.txt b/new name.txt\nsimilarity index 90%\nrename from old name.txt\nrename to new name.txt\n", "old name.txt", "new name.txt", vcs.Renamed},
		{"rename quoted", "diff --git \"a/\\303\\251.go\" b/e.go\nsimilarity index 90%\nrename from \"\\303\\251.go\"\nrename to e.go\n", "é.go", "e.go", vcs.Renamed},
		{"mixed quote", "diff --git a/e.go \"b/\\303\\251.go\"\nsimilarity index 80%\nrename from e.go\nrename to \"\\303\\251.go\"\n", "e.go", "é.go", vcs.Renamed},
		{"plain unified", "--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1 @@\n+hi\n", "", "new.txt", vcs.Added},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := mustParse(t, tt.raw)
			if len(files) != 1 {
				t.Fatalf("files = %d", len(files))
			}
			f := files[0]
			if f.OldPath != tt.old || f.NewPath != tt.new || f.Status != tt.status {
				t.Errorf("got old=%q new=%q status=%s", f.OldPath, f.NewPath, f.Status)
			}
		})
	}
}

func TestParsePlainMultiFile(t *testing.T) {
	raw := "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n--- a/y\n+++ b/y\n@@ -1 +1 @@\n-c\n+d\n"
	files := mustParse(t, raw)
	if len(files) != 2 || files[0].Path() != "x" || files[1].Path() != "y" {
		t.Fatalf("files = %+v", files)
	}
}

func TestParseGitBinaryPatch(t *testing.T) {
	raw := "diff --git a/b.bin b/b.bin\nnew file mode 100644\nindex 0..1\nGIT binary patch\nliteral 4\nLcmZQz00000\n\nliteral 0\nHcmV?d00001\n\n" +
		"diff --git a/c.txt b/c.txt\n--- a/c.txt\n+++ b/c.txt\n@@ -1 +1 @@\n-a\n+b\n"
	files := mustParse(t, raw)
	if len(files) != 2 || !files[0].Binary || files[0].Status != vcs.Added || files[1].Added != 1 {
		t.Fatalf("files = %+v", files)
	}
}

func TestParseModeOnly(t *testing.T) {
	files := mustParse(t, "diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n")
	if len(files) != 1 || !files[0].ModeOnly() {
		t.Fatalf("files = %+v", files)
	}
}

func TestParseDeletedFileHunk(t *testing.T) {
	raw := "diff --git a/d.txt b/d.txt\ndeleted file mode 100644\n--- a/d.txt\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n"
	f := mustParse(t, raw)[0]
	if f.Status != vcs.Deleted || f.NewPath != "" || f.Path() != "d.txt" || f.Deleted != 2 {
		t.Fatalf("f = %+v", f)
	}
}

func TestParseEmptyAndGarbage(t *testing.T) {
	if files, err := Parse(""); err != nil || files != nil {
		t.Errorf("empty: %v %v", files, err)
	}
	files, err := Parse("From: someone\nSubject: patch\n\nhello\n")
	if err != nil || len(files) != 0 {
		t.Errorf("garbage: %v %v", files, err)
	}
}

func TestParseErrors(t *testing.T) {
	for _, raw := range []string{
		"diff --git\n",
		"diff --git garbage\n",
		"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ nonsense @@\n",
	} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) = nil error", raw)
		}
	}
}

// TestParseNeverPanics feeds truncated and mutated input.
func TestParseNeverPanics(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for i := 0; i <= len(s); i++ {
		_, _ = Parse(s[:i])
		_, _ = Parse(s[i:])
	}
	for _, in := range []string{
		"@@ -1,5 +1,5 @@\n", "@@ -1 +1 @@\n+\n+\n+\n", "diff --git a/x b/x\n@@ -1,999999999 +1 @@\n-a\n",
		"diff --git a/x b/x\n@@ -1,2 +1,2 @@\n\\ No newline\n\\ No newline\n", "--- \n+++ \n", "diff --git \"a/\\",
		"diff --git a/x b/x\n@@ -1,1 +1,1 @@\n+a\n-b\n+c\n",
	} {
		_, _ = Parse(in)
	}
}

func TestUnquoteC(t *testing.T) {
	tests := []struct {
		in, val, rest string
		ok            bool
	}{
		{`"abc" tail`, "abc", " tail", true},
		{`"\303\251"`, "é", "", true},
		{`"a\nb\\c\"d"`, "a\nb\\c\"d", "", true},
		{`"\7"`, "\a", "", true},
		{`"unterminated`, "", `"unterminated`, false},
		{`"bad\`, "", `"bad\`, false},
		{`plain`, "", "plain", false},
	}
	for _, tt := range tests {
		v, r, ok := unquoteC(tt.in)
		if v != tt.val || r != tt.rest || ok != tt.ok {
			t.Errorf("unquoteC(%q) = %q,%q,%v", tt.in, v, r, ok)
		}
	}
}

func TestSummary(t *testing.T) {
	n, a, d := Summary([]File{{Added: 3, Deleted: 1}, {Added: 2}})
	if n != 2 || a != 5 || d != 1 {
		t.Errorf("got %d %d %d", n, a, d)
	}
}
