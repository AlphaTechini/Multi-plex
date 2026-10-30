package kit

import (
	"fmt"
	"image/color"
	"path"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// BannerOptions describes a file Banner.
type BannerOptions struct {
	// Width is the exact width in cells.
	Width int
	// Path is the file path relative to the workspace root, with "/"
	// separators as shown to users.
	Path string
	// Index (1-based) and Total place the file in the change set ("3 of 7").
	Index, Total int
	// Status is the change kind ("modified", "added", "deleted", "renamed").
	Status string
	// Added and Removed are line counts.
	Added, Removed int
	// Reviewed shows the file's reviewed check.
	Reviewed bool
	// Settle is the switch animation progress: 0 right after switching files
	// (accent tint), 1 at rest. Drive it over t.Motion.BannerHold; with
	// reduced motion pass 1.
	Settle float64
}

// Banner renders the file header of the diff view: the path with the file
// name in strong text, the position in the change set, the status, line
// counts and reviewed state. Right after a file switch it carries an accent
// tint that settles to the raised surface, so a file change is never missed.
func Banner(t *theme.Theme, o BannerOptions) string {
	w := o.Width
	if w <= 0 {
		return ""
	}
	settle := min(max(o.Settle, 0), 1)
	bg := t.Surface.Raised
	barC := t.Text.Faint
	switch {
	case settle >= 1:
	case t.Animated():
		e := theme.EaseOutCubic(settle)
		bg = theme.Blend(t.Accent.Subtle, t.Surface.Raised, e)
		barC = theme.Blend(t.Accent.Base, t.Text.Faint, e)
	default:
		bg, barC = t.Accent.Subtle, t.Accent.Base
	}

	dir, file := path.Split(o.Path)
	right := bannerMeta(t, o)
	pos := ""
	if o.Total > 0 {
		pos = fg(t.Text.Muted).paint(fmt.Sprintf("  %s  %d of %d", t.Glyphs.Dot, o.Index, o.Total))
	}
	name := ink{fg: t.Text.Strong, bold: true}.paint(file)
	room := w - 3 - Width(right) - 1 - Width(name) - Width(pos)
	dirS := ""
	if dir != "" {
		dirS = fg(t.Text.Secondary).paint(TruncateLeft(dir, max(room, 0), t.Glyphs.Ellipsis))
	}
	left := fg(barC).paint(t.Glyphs.Bar) + " " + dirS + name + pos
	return Backfill(spread(left, right, w-1)+" ", bg)
}

func bannerMeta(t *theme.Theme, o BannerOptions) string {
	var parts []string
	if o.Status != "" {
		parts = append(parts, fg(statusColor(t, o.Status)).paint(o.Status))
	}
	if o.Added > 0 || o.Removed > 0 {
		parts = append(parts, fg(t.Diff.AddSign).paint(fmt.Sprintf("+%d", o.Added))+" "+
			fg(t.Diff.DelSign).paint(fmt.Sprintf("%s%d", t.Glyphs.Minus, o.Removed)))
	}
	if o.Reviewed {
		parts = append(parts, fg(t.State.Done).paint(t.Glyphs.Check+" reviewed"))
	} else {
		parts = append(parts, fg(t.Text.Faint).paint(t.Glyphs.Idle+" not reviewed"))
	}
	return strings.Join(parts, "   ")
}

// statusColor maps a file change kind to its colour.
func statusColor(t *theme.Theme, status string) color.Color {
	switch strings.ToLower(status) {
	case "added", "a", "untracked":
		return t.Diff.AddSign
	case "deleted", "d":
		return t.Diff.DelSign
	case "renamed", "r":
		return t.Diff.Renamed
	default:
		return t.Diff.Modified
	}
}
