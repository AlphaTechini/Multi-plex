package diffview

import (
	"math/rand"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// TestRandomInput throws a long, seeded sequence of keys, clicks, wheel
// events, resizes and refreshes at the view and checks it never panics and
// always renders exactly the size it is given.
func TestRandomInput(t *testing.T) {
	keys := []string{"j", "k", "J", "K", "V", "g", "G", "n", "N", "[", "]", "f", "v", "x", "s", "c", "e", "d", "y", "S", "h", "l",
		"ctrl+d", "ctrl+u", "pgdown", "pgup", "enter", "esc", "down", "up", "left", "right", "home", "end", "tab", "ctrl+s", "a", "b"}
	themes := []*theme.Theme{theme.DarkPlus(), reduced()}
	for seed := int64(1); seed <= 3; seed++ {
		rng := rand.New(rand.NewSource(seed))
		be := mockupFake()
		full := be.raw
		h := newHarness(t, themes[int(seed)%2], be)
		sizes := [][2]int{{112, 45}, {99, 30}, {198, 46}, {60, 12}, {140, 20}, {110, 45}}
		w, hh := 112, 45
		for step := 0; step < 500; step++ {
			switch r := rng.Intn(100); {
			case r < 60:
				k := keys[rng.Intn(len(keys))]
				if h.m.Capturing() && rng.Intn(3) > 0 {
					h.typeText("word ")
				} else {
					h.send(keyMsg(k))
				}
			case r < 70:
				h.send(click(rng.Intn(w+4), rng.Intn(hh+2), tea.KeyMod(rng.Intn(2))*tea.ModShift))
			case r < 76:
				h.send(wheel(rng.Intn(w), rng.Intn(hh), rng.Intn(2) == 0))
			case r < 80:
				h.send(tea.MouseMotionMsg{X: rng.Intn(w), Y: rng.Intn(hh), Button: tea.MouseLeft})
			case r < 86:
				s := sizes[rng.Intn(len(sizes))]
				w, hh = s[0], s[1]
			case r < 90:
				if rng.Intn(2) == 0 {
					be.raw = full
				} else {
					be.raw = strings.Replace(full, "+\ts.metrics.Refreshes.Inc()", "+\ts.metrics.Refreshes.Add(3)", 1)
				}
				h.refresh()
			case r < 93:
				h.clock.advance(100_000_000)
				h.send(view.TickMsg{})
			default:
			}
			out := h.render(w, hh)
			assertBlock(t, "random", out, w, hh)
			if t.Failed() {
				t.Fatalf("seed %d step %d", seed, step)
			}
		}
	}
}
