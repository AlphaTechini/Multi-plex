package kit

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// stringWidth must agree with ansi.StringWidth on every input.
func TestStringWidthMatchesANSI(t *testing.T) {
	corpus := []string{
		"", "plain ascii", "\x1b[31mred\x1b[m", "\x1b]8;;https://x\x07link\x1b]8;;\x07",
		"╭─ auth ─╮ │ ⠹ · … › ▌ ▎ ━ ⌃␣ ↵ ⎇ ✓ ✕ ● ○ ◐ ◌", "世界 日本語 한국어 ｶﾞ ｶﾞ", "é café",
		"👍🏽 👨‍👩‍👧 🇩🇪 ❤️ ☺︎", "\tTab\r\n\x00\x7f", "\x1b[38;2;1;2;3;48;5;234m x \x1b[0m", "\x1b", "\x1b[", "\x1b[31", "\x1b]0;t",
		"\x1b(B ascii", "\xff\xfe broken utf8", "​ zero‍width️", "Ελληνικά Русский", "ก่อน ภาษาไทย", "मराठी",
		"­ soft", "゙゚", "각 jamo", "  sep", "\x9b c1",
	}
	for _, s := range corpus {
		if got, want := stringWidth(s), ansi.StringWidth(s); got != want {
			t.Errorf("stringWidth(%q) = %d, ansi.StringWidth = %d", s, got, want)
		}
	}
	// Every BMP rune, alone and between ASCII letters.
	for r := rune(0); r <= 0xFFFF; r++ {
		for _, s := range []string{string(r), "a" + string(r) + "b", string(r) + string(r)} {
			if got, want := stringWidth(s), ansi.StringWidth(s); got != want {
				t.Fatalf("stringWidth(%q U+%04X) = %d, ansi.StringWidth = %d", s, r, got, want)
			}
		}
	}
	// Random mixes of the corpus.
	rng := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		var b strings.Builder
		for range 1 + rng.IntN(6) {
			p := corpus[rng.IntN(len(corpus))]
			if p != "" && rng.IntN(3) == 0 {
				rs := []rune(p)
				p = string(rs[rng.IntN(len(rs)):])
			}
			b.WriteString(p)
		}
		s := b.String()
		if got, want := stringWidth(s), ansi.StringWidth(s); got != want {
			t.Fatalf("stringWidth(%q) = %d, ansi.StringWidth = %d", s, got, want)
		}
	}
}

// truncate must agree with ansi.Truncate on every input.
func TestTruncateMatchesANSI(t *testing.T) {
	corpus := []string{
		"plain ascii text", "\x1b[31mred\x1b[m and \x1b[1mbold\x1b[0m", "╭─ auth ─╮ │ ⠹ · … › ▌", "世界 日本語 한국어",
		"é café", "👍🏽 🇩🇪", "\ttab", "\x1b]8;;u\x07link\x1b]8;;\x07", "\x1b[38;2;1;2;3m世\x1b[48;5;4m界x\x1b[m",
		"\x7fdel", "\x1b[31", "a\x1bb",
	}
	rng := rand.New(rand.NewPCG(3, 4))
	check := func(s string) {
		for w := 0; w <= stringWidth(s)+1; w++ {
			for _, tail := range []string{"", "…", "...", "世"} {
				if got, want := truncate(s, w, tail), ansi.Truncate(s, w, tail); got != want {
					t.Fatalf("truncate(%q, %d, %q) = %q, ansi.Truncate = %q", s, w, tail, got, want)
				}
			}
		}
	}
	for _, s := range corpus {
		check(s)
	}
	for range 3000 {
		var b strings.Builder
		for range 1 + rng.IntN(5) {
			b.WriteString(corpus[rng.IntN(len(corpus))])
		}
		check(b.String())
	}
}

// resetsBackground must agree with the reference parse over parameter lists.
func TestResetsBackground(t *testing.T) {
	ref := func(p string) bool {
		parts := sgrParams(p)
		for i := 0; i < len(parts); i++ {
			switch parts[i] {
			case "0", "", "49":
				return true
			case "38", "48", "58":
				i += colorArgs(parts, i)
			}
		}
		return false
	}
	atoms := []string{"0", "", "1", "49", "38", "48", "58", "5", "2", "0", "234", "49", "7", "39"}
	rng := rand.New(rand.NewPCG(5, 6))
	for range 50000 {
		n := rng.IntN(8)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = atoms[rng.IntN(len(atoms))]
		}
		p := strings.Join(parts, ";")
		if got, want := resetsBackground(p), ref(p); got != want {
			t.Fatalf("resetsBackground(%q) = %v, want %v", p, got, want)
		}
	}
}
