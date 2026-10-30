package theme

// Box is a set of box-drawing characters for one frame style.
type Box struct {
	// TopLeft, TopRight, BottomLeft and BottomRight are the corners.
	TopLeft, TopRight, BottomLeft, BottomRight string
	// Horizontal and Vertical are the frame edges.
	Horizontal, Vertical string
}

// Glyphs is the symbol vocabulary of the UI. Every entry is exactly one cell
// wide, except Spinner frames which are also one cell each. Use
// UnicodeGlyphs by default and ASCIIGlyphs when the terminal or font lacks
// the symbols (config setting, or a legacy code page).
type Glyphs struct {
	// Unicode reports whether this is the Unicode set.
	Unicode bool

	// Spinner is the frame sequence for the working state.
	Spinner []string
	// Starting, Working, Blocked, Done, Idle and Error are the static state
	// glyphs. Working is used instead of the spinner with reduced motion.
	Starting, Working, Blocked, Done, Idle, Error string

	// Dot is the inline separator ("a · b").
	Dot string
	// Ellipsis ends truncated text.
	Ellipsis string
	// ChevronRight and ChevronLeft mark prompts, breadcrumbs and next/previous.
	ChevronRight, ChevronLeft string
	// Expanded and Collapsed mark tree and section disclosure.
	Expanded, Collapsed string
	// ArrowUp, ArrowDown, ArrowLeft and ArrowRight are used in key hints.
	ArrowUp, ArrowDown, ArrowLeft, ArrowRight string
	// Check marks reviewed and completed items; Cross marks failures.
	Check, Cross string
	// Plus and Minus are diff signs.
	Plus, Minus string
	// Branch prefixes git branch names.
	Branch string
	// Bar is the thick left accent bar of cards, banners and selected rows.
	Bar string
	// BarThin is the thin left bar of comments and quotes.
	BarThin string
	// Fold is the dot used in collapsed-region rows.
	Fold string
	// ProgressFill and ProgressTrack draw thin progress bars.
	ProgressFill, ProgressTrack string
	// Rule is the light horizontal rule; RuleHeavy marks the active tab.
	Rule, RuleHeavy string
	// Frame is the panel frame (rounded, light lines).
	Frame Box

	// Ctrl, Shift, Alt, Enter, Esc, Space, Tab and Backspace render key names
	// in key hints.
	Ctrl, Shift, Alt, Enter, Esc, Space, Tab, Backspace string
}

// UnicodeGlyphs returns the default symbol set: rounded light box drawing,
// braille spinner, geometric state dots.
func UnicodeGlyphs() Glyphs {
	return Glyphs{
		Unicode:       true,
		Spinner:       []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		Starting:      "◌",
		Working:       "◐",
		Blocked:       "●",
		Done:          "✓",
		Idle:          "○",
		Error:         "✕",
		Dot:           "·",
		Ellipsis:      "…",
		ChevronRight:  "›",
		ChevronLeft:   "‹",
		Expanded:      "▾",
		Collapsed:     "▸",
		ArrowUp:       "↑",
		ArrowDown:     "↓",
		ArrowLeft:     "←",
		ArrowRight:    "→",
		Check:         "✓",
		Cross:         "✕",
		Plus:          "+",
		Minus:         "−",
		Branch:        "⎇",
		Bar:           "▌",
		BarThin:       "▎",
		Fold:          "·",
		ProgressFill:  "━",
		ProgressTrack: "─",
		Rule:          "─",
		RuleHeavy:     "━",
		Frame: Box{
			TopLeft: "╭", TopRight: "╮", BottomLeft: "╰", BottomRight: "╯",
			Horizontal: "─", Vertical: "│",
		},
		Ctrl:      "⌃",
		Shift:     "⇧",
		Alt:       "⌥",
		Enter:     "↵",
		Esc:       "esc",
		Space:     "␣",
		Tab:       "tab",
		Backspace: "⌫",
	}
}
