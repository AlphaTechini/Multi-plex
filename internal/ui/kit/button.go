package kit

import (
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// ButtonVariant is a button's visual weight.
type ButtonVariant int

// Button variants.
const (
	// ButtonSecondary is a neutral filled button.
	ButtonSecondary ButtonVariant = iota
	// ButtonPrimary is the accent-filled default action. Use at most one per
	// dialog.
	ButtonPrimary
	// ButtonGhost is text only, for low-weight actions.
	ButtonGhost
)

// ButtonOptions describes a Button.
type ButtonOptions struct {
	// Label is the action ("Approve once").
	Label string
	// Keys is the key spec that triggers it (see FormatKeys).
	Keys string
	// Variant sets the visual weight.
	Variant ButtonVariant
}

// Button renders a one-line button: " Label  key " on a filled surface. The
// key is part of the button so every action shows how to trigger it.
func Button(t *theme.Theme, o ButtonOptions) string {
	keys := FormatKeys(t.Glyphs, o.Keys)
	var label, key ink
	switch o.Variant {
	case ButtonPrimary:
		label = ink{fg: t.Text.OnAccent, bg: t.Accent.Fill, bold: true}
		key = ink{fg: theme.Blend(t.Text.OnAccent, t.Accent.Fill, 0.35), bg: t.Accent.Fill}
	case ButtonGhost:
		label = ink{fg: t.Text.Secondary}
		key = ink{fg: t.Text.Muted}
	default:
		label = ink{fg: t.Text.Primary, bg: t.Surface.Control}
		key = ink{fg: t.Text.Muted, bg: t.Surface.Control}
	}
	if keys == "" {
		return label.paint(" " + o.Label + " ")
	}
	return label.paint(" "+o.Label) + key.paint("  "+keys+" ")
}
