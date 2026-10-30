package kit

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Tab is one main-view mode tab.
type Tab struct {
	// Label is the mode name, for example "Diff".
	Label string
	// Badge is an optional count shown after the label ("7").
	Badge string
	// Alert colours the badge as a warning (something there needs you).
	Alert bool
}

// TabBarOptions describes a TabBar.
type TabBarOptions struct {
	// Tabs in display order.
	Tabs []Tab
	// Active is the index of the selected tab.
	Active int
	// Width > 0 fits the bar to exactly Width cells; 0 keeps its natural
	// width (for use as a Panel Header).
	Width int
}

// TabBar renders the mode tabs on one line. Every tab has one cell of padding
// each side, so selecting a tab never shifts the others; the active tab is a
// pill on the accent tint with strong, bold text, the others are muted.
func TabBar(t *theme.Theme, o TabBarOptions) string {
	var b strings.Builder
	for i, tab := range o.Tabs {
		if i > 0 {
			b.WriteString(" ")
		}
		if i == o.Active {
			pill := ink{fg: t.Text.Strong, bg: t.Accent.Subtle, bold: true}
			b.WriteString(pill.paint(" " + tab.Label))
			if tab.Badge != "" {
				bc := t.Text.Secondary
				if tab.Alert {
					bc = t.Severity.Warning
				}
				b.WriteString(ink{fg: bc, bg: t.Accent.Subtle}.paint(" " + tab.Badge))
			}
			b.WriteString(pill.paint(" "))
			continue
		}
		b.WriteString(fg(t.Text.Muted).paint(" " + tab.Label))
		if tab.Badge != "" {
			bc := t.Text.Faint
			if tab.Alert {
				bc = t.Severity.Warning
			}
			b.WriteString(fg(bc).paint(" " + tab.Badge))
		}
		b.WriteString(" ")
	}
	if o.Width > 0 {
		return Fit(b.String(), o.Width)
	}
	return b.String()
}
