package syntax

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/highlight/chromahl"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

func goSource(n int) string {
	var b strings.Builder
	b.WriteString("package x\n\n/* a block\ncomment */\n")
	for i := range n {
		fmt.Fprintf(&b, "func f%d() int { return %d } // c\n", i, i)
	}
	return b.String()
}

// run executes a command (and batches) synchronously, returning ChunkMsgs.
func run(cmd tea.Cmd) []ChunkMsg {
	if cmd == nil {
		return nil
	}
	switch m := cmd().(type) {
	case ChunkMsg:
		return []ChunkMsg{m}
	case tea.BatchMsg:
		var out []ChunkMsg
		for _, c := range m {
			out = append(out, run(c)...)
		}
		return out
	}
	return nil
}

func TestSourceLines(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{{"", 0}, {"\n", 1}, {"a", 1}, {"a\n", 1}, {"a\r\nb\r\n", 2}, {"a\n\n", 2}}
	for _, c := range cases {
		s := NewSource(nil, "a.go", c.in)
		if s.NumLines() != c.want {
			t.Errorf("%q: %d lines, want %d", c.in, s.NumLines(), c.want)
		}
	}
	s := NewSource(nil, "a.go", "a\r\nb\r\n")
	if s.Text(0) != "a" || s.Text(1) != "b" || s.Text(2) != "" || s.Text(-1) != "" {
		t.Errorf("Text: %q %q", s.Text(0), s.Text(1))
	}
	if s.Request(0, 10) != nil {
		t.Error("nil highlighter should give no command")
	}
}

func TestRequestWindowAndApply(t *testing.T) {
	// Larger than fullLimit: only chunk jobs.
	src := NewSource(chromahl.New(), "main.go", goSource(fullLimit+500))
	if src.Language() != "go" {
		t.Fatalf("language %q", src.Language())
	}
	if src.Ready(300) {
		t.Fatal("ready before any job")
	}
	msgs := run(src.Request(300, 330))
	// chunk 2 (256-383) plus one each side.
	if len(msgs) != 3 {
		t.Fatalf("got %d chunks, want 3", len(msgs))
	}
	if again := src.Request(300, 330); again != nil {
		t.Error("in-flight chunks must not be requested twice")
	}
	for _, m := range msgs {
		if !src.Apply(m) {
			t.Error("Apply rejected own message")
		}
	}
	if !src.Ready(300) || src.Ready(0) || src.Ready(5000) {
		t.Error("only the requested window should be ready")
	}
	if got := src.Request(300, 330); got != nil {
		t.Error("cached chunks must not be requested again")
	}
	// Every highlighted line reproduces its text.
	for i := 256; i < 384; i++ {
		var sb strings.Builder
		for _, tk := range src.Tokens(i) {
			sb.WriteString(tk.Text)
		}
		if sb.String() != src.Text(i) {
			t.Fatalf("line %d: tokens %q != text %q", i, sb.String(), src.Text(i))
		}
	}
	if toks := src.Tokens(300); len(toks) == 0 || toks[0].Kind != highlight.Keyword {
		t.Errorf("line 300 tokens: %+v", toks)
	}
	other := NewSource(chromahl.New(), "x.go", "package x\n")
	if other.Apply(msgs[0]) {
		t.Error("foreign message accepted")
	}
}

func TestSmallFileOneJob(t *testing.T) {
	src := NewSource(chromahl.New(), "main.go", goSource(100))
	msgs := run(src.Request(0, 20))
	if len(msgs) != 1 || src.Request(0, 20) != nil {
		t.Fatalf("small file: %d jobs", len(msgs))
	}
	src.Apply(msgs[0])
	if !src.Ready(0) || !src.Ready(src.NumLines()-1) {
		t.Error("whole file should be ready")
	}
	if src.Request(0, 20) != nil {
		t.Error("nothing left to request")
	}
}

func TestFullPassReplacesChunksAndIsExact(t *testing.T) {
	// A 300-line comment: chunk windows cannot see its start, the full pass can.
	var b strings.Builder
	b.WriteString("/*\n")
	for range 700 {
		b.WriteString(" inside the comment\n")
	}
	b.WriteString("*/\nfunc f() {}\n")
	src := NewSource(chromahl.New(), "c.go", b.String())
	msgs := run(src.Request(400, 410))
	if len(msgs) < 2 {
		t.Fatalf("want chunk jobs plus a full pass, got %d", len(msgs))
	}
	var full ChunkMsg
	for _, m := range msgs {
		if m.chunk < 0 {
			full = m
		} else {
			src.Apply(m)
		}
	}
	if toks := src.Tokens(405); len(toks) == 0 || toks[0].Kind == highlight.Comment {
		t.Logf("chunk pass happened to be exact: %+v", toks)
	}
	if !src.Apply(full) {
		t.Fatal("full pass rejected")
	}
	if toks := src.Tokens(405); len(toks) == 0 || toks[0].Kind != highlight.Comment {
		t.Errorf("exact pass: line inside a long comment is %+v", toks)
	}
	// A late chunk result must not overwrite the exact tokens.
	for _, m := range msgs {
		if m.chunk >= 0 && src.Apply(m) {
			t.Error("stale chunk applied after the full pass")
		}
	}
}

func TestInheritKeepsUnchangedLines(t *testing.T) {
	h := chromahl.New()
	old := NewSource(h, "a.go", "package a\n\nfunc f() {}\n")
	for _, m := range run(old.Request(0, 5)) {
		old.Apply(m)
	}
	next := NewSource(h, "a.go", "package a\n\nfunc g() {}\nvar x = 1\n")
	if next.Tokens(0) != nil {
		t.Fatal("tokens before Inherit")
	}
	next.Inherit(old)
	if len(next.Tokens(0)) == 0 {
		t.Error("unchanged line should borrow tokens")
	}
	if next.Tokens(2) != nil || next.Tokens(3) != nil {
		t.Error("changed and new lines must not borrow")
	}
	for _, m := range run(next.Request(0, 5)) {
		next.Apply(m)
	}
	if next.prev != nil {
		t.Error("previous version should be released after the exact pass")
	}
	if len(next.Tokens(3)) == 0 {
		t.Error("own tokens missing")
	}
}

func TestColor(t *testing.T) {
	th := theme.DarkPlus()
	cases := []struct {
		k    highlight.Kind
		text string
		want any
	}{
		{highlight.Keyword, "func", th.Syntax.Keyword},
		{highlight.Keyword, "return", th.Syntax.Control},
		{highlight.String, `"x"`, th.Syntax.String},
		{highlight.Comment, "//", th.Syntax.Comment},
		{highlight.Operator, "+", th.Syntax.Plain},
		{highlight.Type, "int", th.Syntax.Type},
		{highlight.Function, "f", th.Syntax.Function},
		{highlight.Number, "1", th.Syntax.Number},
		{highlight.Constant, "nil", th.Syntax.Constant},
		{highlight.Variable, "x", th.Syntax.Variable},
	}
	for _, c := range cases {
		if got := Color(th, c.k, c.text); got != c.want {
			t.Errorf("Color(%v,%q) = %v, want %v", c.k, c.text, got, c.want)
		}
	}
	sp := Spans(th, []highlight.Token{{Text: "if", Kind: highlight.Keyword}})
	if len(sp) != 1 || sp[0].Text != "if" || sp[0].Fg != th.Syntax.Control {
		t.Errorf("Spans: %+v", sp)
	}
}

func BenchmarkRequestChunk(b *testing.B) {
	content := goSource(2000)
	h := chromahl.New()
	for b.Loop() {
		src := NewSource(h, "main.go", content)
		for _, m := range run(src.Request(600, 640)) {
			src.Apply(m)
		}
	}
}
