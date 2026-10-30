// Command mpx-preview renders Multiplex's screens with the real theme and kit
// primitives and fake data, so the design can be checked in a real terminal
// without the daemon or Bubble Tea.
//
//	go run ./cmd/mpx-preview -screen diff -w 180 -h 48
//
// Screens: overview, diff, approval, palette, empty, newsession, kit, and
// tokens (the colour token table as Markdown).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/colorprofile"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

func main() {
	w := flag.Int("w", 180, "screen width in cells")
	h := flag.Int("h", 48, "screen height in cells")
	screen := flag.String("screen", "overview", "overview|diff|approval|palette|empty|newsession|kit|tokens")
	ascii := flag.Bool("ascii", false, "use the ASCII glyph fallback set")
	profile := flag.String("profile", "truecolor", "colour profile: truecolor|256")
	frame := flag.Int("frame", 2, "spinner frame index")
	reduce := flag.Bool("reduce-motion", false, "render with reduced motion")
	settle := flag.Float64("settle", 1, "diff banner settle progress 0..1 (0 = just switched files)")
	fade := flag.Float64("fade", 1, "diff content fade-in progress 0..1")
	fresh := flag.Float64("fresh", 0, "new-hunk highlight 0..1 on the diff's second hunk (live update)")
	plain := flag.Bool("plain", false, "strip colour (for the ASCII mockups in docs/16)")
	themeName := flag.String("theme", theme.DarkPlusName, "theme name")
	flag.Parse()

	t, ok := theme.ByName(*themeName)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown theme %q (have %v)\n", *themeName, theme.Names())
		os.Exit(2)
	}
	if *profile == "256" {
		t = t.For(colorprofile.ANSI256)
	}
	if *ascii {
		t = t.WithGlyphs(theme.ASCIIGlyphs())
	}
	t = t.WithReducedMotion(*reduce)

	s := scene{t: t, w: *w, h: *h, frame: *frame, settle: *settle, fade: *fade, fresh: *fresh}
	out, err := s.render(*screen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *plain {
		out = stripANSI(out)
	}
	fmt.Println(out)
}
