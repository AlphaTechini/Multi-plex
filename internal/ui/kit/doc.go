// Package kit holds the visual primitives every Multiplex screen is built
// from: panels, tabs, rows, cards, banners, diff lines, modals, toasts, the
// status bar and the text helpers under them. The design they implement is
// specified in docs/16-visual-design.md; read it before adding a primitive.
//
// # Conventions
//
// Pure functions. A primitive takes a *theme.Theme and an options struct
// (data plus size) and returns a string. It holds no Bubble Tea state, does
// no I/O and never blocks, so it is safe to call from View on every frame.
//
// Sizes are exact. When a primitive is given a width, every line it returns
// is exactly that many terminal cells; when it is given a height, it returns
// exactly that many lines. Text is clipped with an ellipsis and padded with
// spaces, never wrapped past the box. This is what keeps the layout from
// jittering when content changes, and the tests assert it for widths 0 to
// 120. A width or height of 0 or less returns an empty string instead of
// panicking. A few primitives (TabBar, KeyHints, Choice, ButtonRow) also
// accept width 0 to mean "natural width", which is documented on each.
//
// Widths are measured in cells, not bytes or runes: escape sequences take no
// cells and wide characters (CJK, most emoji) take two. Always use Width,
// Truncate, TruncateLeft, PadRight, PadLeft, Fit, FitBlock and Wrap from this
// package, never len(). They give exactly the charmbracelet/x/ansi answers
// but take a fast path for the text the UI draws (ASCII, escape sequences,
// box drawing, symbols, CJK) and only run the grapheme segmenter for text
// that needs it, which keeps a full 180×48 frame under a millisecond.
//
// Colours come from the theme. Primitives take semantic inputs (State,
// Severity, Tone, DiffKind) and look colours up in the theme; screens should
// do the same and never write hex values. The only colours a caller passes
// directly are syntax colours inside Span, and those come from t.Syntax.
//
// Backgrounds are painted. Every cell a primitive returns carries its
// surface colour, so the app looks the same on any terminal background.
// Content rendered elsewhere (the agent's terminal, a bubbles component)
// should go through Backfill, which re-applies the surface after every reset
// inside it. Styled strings can be nested freely: an outer style is restored
// after an inner reset.
//
// Structure without boxes. Panels (rounded, light frames) sit side by side
// and are never nested. Inside a panel, hierarchy comes from spacing, the
// text tiers (Strong, Primary, Secondary, Muted, Faint), Section headers,
// thin rules and raised surfaces with a left bar (cards, comments, toasts),
// not from more frames. Only one region is focused at a time, and only the
// focused region uses the focus colour on its frame.
//
// Overlays are composited. Modal dims the screen (Dim) and draws its box
// over it with Composite; Toast and the command palette use the same two
// functions. Composite cuts lines by cells and handles wide characters at
// the edges.
//
// Motion is opt-in per frame. Primitives that animate (StateDot for the
// spinner, Banner for the file-switch tint, DiffLine for the new-hunk
// highlight, Dim for the fade) take the animation progress as a plain
// number; the caller owns the clock and the tick messages. Check
// t.Animated() before scheduling ticks: with reduced motion, or below
// truecolor, primitives fall back to static forms.
//
// One primitive per file, named after it (panel.go holds Panel and
// PanelOptions).
package kit
