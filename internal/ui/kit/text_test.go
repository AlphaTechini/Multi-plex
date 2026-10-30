package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const styled = "\x1b[31mab\x1b[44m世界cd\x1b[m ef" // 11 cells, wide chars at 2..5

func TestTruncate(t *testing.T) {
	for w := -1; w <= 12; w++ {
		for _, tail := range []string{"", "…", "..."} {
			got := Truncate(styled, w, tail)
			if gw := ansi.StringWidth(got); gw > max(w, 0) {
				t.Errorf("Truncate(%d,%q) = %d cells", w, tail, gw)
			}
		}
	}
	if got := Truncate("hello", 5, "…"); got != "hello" {
		t.Errorf("Truncate kept-length = %q", got)
	}
	if got := ansi.Strip(Truncate("hello world", 6, "…")); got != "hello…" {
		t.Errorf("Truncate = %q, want hello…", got)
	}
}

func TestTruncateLeft(t *testing.T) {
	for w := -1; w <= 12; w++ {
		got := TruncateLeft(styled, w, "…")
		if gw := ansi.StringWidth(got); gw > max(w, 0) {
			t.Errorf("TruncateLeft(%d) = %d cells: %q", w, gw, got)
		}
	}
	if got := TruncateLeft("src/auth/session.go", 12, "…"); ansi.Strip(got) != "…/session.go" {
		t.Errorf("TruncateLeft path = %q", ansi.Strip(got))
	}
}

func TestFitAndPad(t *testing.T) {
	for _, w := range widths {
		for _, s := range []string{"", "x", styled, "世界世界世界", strings.Repeat("abc ", 50)} {
			assertWidth(t, "Fit", Fit(s, w), w)
			assertWidth(t, "Center", Center(s, w), w)
		}
	}
	if got := PadRight("ab", 4); got != "ab  " {
		t.Errorf("PadRight = %q", got)
	}
	if got := PadLeft("ab", 4); got != "  ab" {
		t.Errorf("PadLeft = %q", got)
	}
	if got := PadRight("abcdef", 3); got != "abcdef" {
		t.Errorf("PadRight must not truncate, got %q", got)
	}
}

func TestFitBlock(t *testing.T) {
	for _, w := range widths {
		for h := 0; h < 6; h++ {
			assertBlock(t, "FitBlock", FitBlock("a\n"+styled+"\nc\nd\ne\nf\ng", w, h), w, h)
		}
	}
}

func TestWidthMultiline(t *testing.T) {
	if got := Width("ab\nabcd\n" + styled); got != 11 {
		t.Errorf("Width = %d, want 11", got)
	}
}

func TestBackfill(t *testing.T) {
	th := themes()["truecolor"]
	in := "a\x1b[31mb\x1b[mc\x1b[0md\x1b[38;5;0me\x1b[49mf"
	out := Backfill(in, th.Surface.Base)
	if ansi.Strip(out) != ansi.Strip(in) {
		t.Errorf("Backfill changed text: %q", ansi.Strip(out))
	}
	bg := ink{bg: th.Surface.Base}.seq()
	// Initial bg, after "\x1b[m", "\x1b[0m" and "\x1b[49m", but not after 38;5;0.
	if n := strings.Count(out, bg); n != 4 {
		t.Errorf("Backfill applied bg %d times, want 4: %q", n, out)
	}
	if Backfill("x", nil) != "x" {
		t.Error("Backfill(nil) must not change the input")
	}
	noLeak(t, "Backfill", out)
}

func TestCompositeKeepsSize(t *testing.T) {
	th := themes()["truecolor"]
	// Wide characters everywhere, so every edge of fg cuts one.
	bg := Backfill(strings.Repeat("世界世界世界世界世界\n", 5)+"世界世界世界世界世界", th.Surface.Base)
	fg := Backfill(FitBlock("box", 7, 3), th.Surface.Overlay)
	for x := -9; x <= 22; x++ {
		for y := -4; y <= 7; y++ {
			out := Composite(bg, fg, x, y)
			assertBlock(t, "Composite", out, 20, 6)
		}
	}
	out := ansi.Strip(Composite("..........\n..........", "XY", 3, 1))
	if out != "..........\n...XY....." {
		t.Errorf("Composite = %q", out)
	}
}

func TestJoinColumns(t *testing.T) {
	out := JoinColumns("ab\nc", "1\n22\n333")
	assertBlock(t, "JoinColumns", out, 5, 3)
	if JoinColumns() != "" {
		t.Error("JoinColumns() should be empty")
	}
}

func TestDim(t *testing.T) {
	th := themes()["truecolor"]
	in := Panel(th, PanelOptions{Width: 20, Height: 4, Title: "x", Content: "\x1b[31mred\x1b[m \x1b[38;5;45mi\x1b[39m"})
	out := Dim(th, in, th.Surface.Sunken, 0.7)
	if ansi.Strip(out) != ansi.Strip(in) {
		t.Error("Dim changed text")
	}
	assertBlock(t, "Dim", out, 20, 4)
	if Dim(th, in, th.Surface.Sunken, 0) != in {
		t.Error("Dim(0) must return the input")
	}
	noLeak(t, "Dim", out)
}

func TestFormatKeys(t *testing.T) {
	th := themes()["truecolor"]
	cases := map[string]string{
		"ctrl+space p": "⌃␣ p",
		"ctrl+\\ p":    "⌃\\ p",
		"shift+tab":    "⇧tab",
		"enter":        "↵",
		"up down":      "↑ ↓",
		"q":            "q",
		"hyper+x":      "hyper+x",
	}
	for in, want := range cases {
		if got := FormatKeys(th.Glyphs, in); got != want {
			t.Errorf("FormatKeys(%q) = %q, want %q", in, got, want)
		}
	}
	if got := FormatKeys(themes()["ascii"].Glyphs, "ctrl+space p"); got != "C-space p" {
		t.Errorf("ASCII FormatKeys = %q", got)
	}
}

func TestPaintNestedKeepsOuterStyle(t *testing.T) {
	th := themes()["truecolor"]
	inner := fg(th.State.Blocked).paint("●")
	out := fg(th.Text.Muted).paint(inner + " name")
	muted := fg(th.Text.Muted).seq()
	// The muted ink must be re-applied after the inner reset, before " name".
	if !strings.Contains(out, ansi.ResetStyle+muted+" name") {
		t.Errorf("outer ink not restored after nested reset: %q", out)
	}
}
