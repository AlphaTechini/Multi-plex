package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// themes are the variants every primitive is tested with.
func themes() map[string]*theme.Theme {
	base := theme.DarkPlus()
	return map[string]*theme.Theme{
		"truecolor": base,
		"256":       base.For(colorprofile.ANSI256),
		"ascii":     base.WithGlyphs(theme.ASCIIGlyphs()),
		"reduced":   base.WithReducedMotion(true),
	}
}

// widths covers degenerate, tiny and normal sizes.
var widths = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 13, 24, 40, 61, 120}

// assertWidth checks that every line of s is exactly w cells.
func assertWidth(t *testing.T, name, s string, w int) {
	t.Helper()
	if w <= 0 {
		if ansi.StringWidth(s) != 0 {
			t.Errorf("%s: width %d: got %d cells, want 0", name, w, ansi.StringWidth(s))
		}
		return
	}
	for i, line := range strings.Split(s, "\n") {
		if got := ansi.StringWidth(line); got != w {
			t.Errorf("%s: line %d is %d cells, want %d: %q", name, i, got, w, ansi.Strip(line))
		}
	}
}

// assertBlock checks that s is exactly h lines of w cells.
func assertBlock(t *testing.T, name, s string, w, h int) {
	t.Helper()
	if h <= 0 || w <= 0 {
		if ansi.StringWidth(s) != 0 {
			t.Errorf("%s: %d×%d: got %d cells, want an empty block", name, w, h, ansi.StringWidth(s))
		}
		return
	}
	if h > 0 && w > 0 {
		if got := len(strings.Split(s, "\n")); got != h {
			t.Errorf("%s: %d lines, want %d", name, got, h)
		}
	}
	assertWidth(t, name, s, w)
}

// noLeak checks that s does not leave styles open at its end.
func noLeak(t *testing.T, name, s string) {
	t.Helper()
	for i, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "\x1b[") && !strings.HasSuffix(line, ansi.ResetStyle) && !strings.HasSuffix(line, "\x1b[0m") {
			// A line may end in plain text after a reset; check the last SGR.
			last := strings.LastIndex(line, "\x1b[")
			if seq := line[last:]; !strings.HasPrefix(seq, ansi.ResetStyle) && !strings.HasPrefix(seq, "\x1b[0m") {
				t.Errorf("%s: line %d leaves a style open: %q", name, i, seq)
			}
		}
	}
}
