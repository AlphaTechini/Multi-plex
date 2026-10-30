package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// Picker returns the colour for one palette entry: the truecolor hex value or
// the explicit 256-colour index, depending on the profile it was made for.
type Picker func(hex string, index256 uint8) color.Color

// NewPicker returns a Picker for the profile. TrueColor (and Unknown) use the
// hex value. ANSI256 and ANSI use the 256-colour index; the terminal writer
// reduces it further to 16 colours on ANSI terminals. ASCII and NoTTY also get
// the index, and the writer strips colour entirely.
func NewPicker(p colorprofile.Profile) Picker {
	if p == colorprofile.TrueColor || p == colorprofile.Unknown {
		return func(hex string, _ uint8) color.Color { return lipgloss.Color(hex) }
	}
	return func(_ string, index256 uint8) color.Color { return ansi.IndexedColor(index256) }
}
