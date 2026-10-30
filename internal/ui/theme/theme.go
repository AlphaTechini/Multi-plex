// Package theme holds Multiplex's design tokens: colours, glyphs, motion and
// the lipgloss styles derived from them.
//
// A Theme is a plain struct of semantic tokens. Screens and the kit package
// read tokens by meaning (t.Text.Muted, t.State.Blocked, t.Diff.AddLineBg) and
// never use raw hex values, so a theme can be replaced by dropping one file
// into this package that calls Register from an init function.
//
// Every colour has a truecolor value and an explicit 256-colour fallback.
// Builders receive the terminal's colour profile and pick one of the two, so
// the 256-colour look is designed, not left to automatic nearest-colour
// conversion.
package theme

import (
	"github.com/charmbracelet/colorprofile"
)

// Theme is a complete set of design tokens plus the styles derived from them.
// Treat a *Theme as read-only once built; use the With* methods to get a
// modified copy.
type Theme struct {
	// Name is the registry name, for example "dark-plus".
	Name string
	// Profile is the colour profile the colours were resolved for.
	Profile colorprofile.Profile

	// Surface holds background colours, from the sunken sidebar to overlays.
	Surface Surfaces
	// Border holds line colours for panel frames, dividers and the focus ring.
	Border Borders
	// Text holds the foreground text tiers.
	Text Texts
	// Accent holds the interactive accent (focus, links, primary buttons).
	Accent Accents
	// State holds session state colours (working, blocked, done...).
	State States
	// Severity holds insight card severity colours.
	Severity Severities
	// Diff holds diff viewer colours.
	Diff DiffColors
	// Syntax holds code token colours for syntax highlighting.
	Syntax SyntaxColors

	// Glyphs is the symbol set in use (Unicode or ASCII fallback).
	Glyphs Glyphs
	// Motion holds animation timings and the reduce-motion switch.
	Motion Motion
	// Styles holds lipgloss styles derived from the tokens above.
	Styles Styles

	build Builder
}

// For returns the same theme resolved for another colour profile, for example
// after Bubble Tea reports the terminal's profile. Glyphs and motion settings
// are kept. It returns t unchanged when the theme has no builder.
func (t *Theme) For(p colorprofile.Profile) *Theme {
	if t.build == nil || p == t.Profile {
		return t
	}
	n := t.build(p)
	n.Glyphs = t.Glyphs
	n.Motion = t.Motion
	n.build = t.build
	n.Styles = NewStyles(n)
	return n
}

// WithGlyphs returns a copy of the theme that uses the given glyph set, for
// example ASCIIGlyphs() on terminals without good Unicode fonts.
func (t *Theme) WithGlyphs(g Glyphs) *Theme {
	n := *t
	n.Glyphs = g
	n.Styles = NewStyles(&n)
	return &n
}

// WithReducedMotion returns a copy of the theme with motion turned off or on.
// With reduced motion, fades, banner animation, hunk highlight decay and the
// spinner are disabled (see Motion).
func (t *Theme) WithReducedMotion(reduce bool) *Theme {
	n := *t
	n.Motion.Reduce = reduce
	return &n
}

// Animated reports whether colour animation (fades, decays) should run. It is
// false with reduced motion and below truecolor, where intermediate colours
// would snap to the same 256-colour index.
func (t *Theme) Animated() bool {
	return !t.Motion.Reduce && t.Profile == colorprofile.TrueColor
}
