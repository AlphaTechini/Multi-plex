package theme

// ASCIIGlyphs returns the fallback symbol set for terminals or fonts without
// the Unicode symbols. It keeps every glyph one cell wide so layouts do not
// change when switching sets.
func ASCIIGlyphs() Glyphs {
	return Glyphs{
		Unicode:       false,
		Spinner:       []string{"|", "/", "-", "\\"},
		Starting:      ".",
		Working:       "~",
		Blocked:       "!",
		Done:          "+",
		Idle:          "o",
		Error:         "x",
		Dot:           "|",
		Ellipsis:      "~",
		ChevronRight:  ">",
		ChevronLeft:   "<",
		Expanded:      "v",
		Collapsed:     ">",
		ArrowUp:       "^",
		ArrowDown:     "v",
		ArrowLeft:     "<",
		ArrowRight:    ">",
		Check:         "+",
		Cross:         "x",
		Plus:          "+",
		Minus:         "-",
		Branch:        "@",
		Bar:           "|",
		BarThin:       "|",
		Fold:          ".",
		ProgressFill:  "=",
		ProgressTrack: "-",
		Rule:          "-",
		RuleHeavy:     "=",
		Frame: Box{
			TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
			Horizontal: "-", Vertical: "|",
		},
		Ctrl:      "C-",
		Shift:     "S-",
		Alt:       "M-",
		Enter:     "enter",
		Esc:       "esc",
		Space:     "space",
		Tab:       "tab",
		Backspace: "bksp",
	}
}
