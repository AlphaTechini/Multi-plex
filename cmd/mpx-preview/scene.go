package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// scene holds what every screen needs: the theme, the screen size and the
// animation inputs.
type scene struct {
	t            *theme.Theme
	w, h         int
	frame        int
	settle, fade float64
	fresh        float64
}

// minWidth and minHeight are the smallest supported terminal (docs/16).
const (
	minWidth  = 80
	minHeight = 24
)

func (s scene) render(name string) (string, error) {
	if name != "tokens" && (s.w < minWidth || s.h < minHeight) {
		return s.tooSmall(), nil
	}
	switch name {
	case "overview":
		return s.overview(), nil
	case "diff":
		return s.diff(), nil
	case "approval":
		return s.approval(), nil
	case "palette":
		return s.palette(), nil
	case "empty":
		return s.empty(), nil
	case "newsession":
		return s.newSession(), nil
	case "kit":
		return s.gallery(), nil
	case "tokens":
		return s.tokens(), nil
	}
	return "", fmt.Errorf("unknown screen %q", name)
}

// tooSmall is shown below the minimum size.
func (s scene) tooSmall() string {
	return kit.EmptyState(s.t, kit.EmptyStateOptions{
		Width: s.w, Height: s.h,
		Title: "Terminal too small",
		Body:  fmt.Sprintf("Multiplex needs at least %d×%d. This one is %d×%d.", minWidth, minHeight, s.w, s.h),
	})
}

func stripANSI(s string) string { return ansi.Strip(s) }

func join(lines ...string) string { return strings.Join(lines, "\n") }
