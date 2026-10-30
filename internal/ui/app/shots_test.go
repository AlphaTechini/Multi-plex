package app

import (
	"os"
	"testing"

	"github.com/alphatechini/multi-plex/internal/ui/backend/fake"
)

// TestShots renders the screens for visual QA when MPX_SHOTS_DIR is set.
func TestShots(t *testing.T) {
	if os.Getenv("MPX_SHOTS_DIR") == "" {
		t.Skip("set MPX_SHOTS_DIR to write ANSI screens")
	}
	for _, sz := range [][2]int{{180, 48}, {110, 32}} {
		w, hgt := sz[0], sz[1]
		name := func(s string) string { return s + "-" + itoa(w) + "x" + itoa(hgt) }

		h := newHarness(t, w, hgt)
		h.m.frame = 2
		h.shot(name("overview"))

		h.keys("ctrl+\\", "p")
		h.shot(name("palette-all"))
		h.typeText("new")
		h.shot(name("palette"))
		h.key("esc")

		h.keys("ctrl+\\", "n")
		h.typeText("Make the token cache per-user and update the callers in api/ and worker/.")
		h.shot(name("newsession"))
		h.key("esc")

		h.keys("ctrl+\\", "!")
		h.shot(name("approval"))
		h.key("esc")
		h.settle()

		h.keys("ctrl+\\", "?")
		h.shot(name("help"))
		h.key("esc")

		h.keys("ctrl+\\", "d")
		h.shot(name("diffmode"))
		h.keys("ctrl+\\", "t")
		h.settle()
		h.shot(name("term"))

		e := newHarness(t, w, hgt, fake.Empty())
		e.shot(name("empty"))
	}
}
