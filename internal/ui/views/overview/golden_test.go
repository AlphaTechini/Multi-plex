package overview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// TestGolden pins the layout of the docs/16 Overview mockups at the main
// view's inner size for 180×48 (90×45) and 110×32 (99×29). The goldens equal
// the mockups' main region except for two data-driven lines: the spinner
// frame and the compact Done sentence. Run with MPX_UPDATE_GOLDEN=1 to
// rewrite them.
func TestGolden(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{{"overview-180x48", 90, 45}, {"overview-110x32", 99, 29}}
	for _, c := range cases {
		m, _ := newFixture(t, theme.DarkPlus())
		got := strings.Join(plain(m.Render(c.w, c.h)), "\n") + "\n"
		path := filepath.Join("testdata", c.name+".golden")
		if os.Getenv("MPX_UPDATE_GOLDEN") != "" {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
			t.Errorf("%s differs from golden:\n%s", c.name, got)
		}
	}
}
