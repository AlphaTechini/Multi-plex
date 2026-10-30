package kit

import "image/color"

// Span is a run of code text with one syntax colour. Diff and code views
// build lines from spans so the kit can lay backgrounds (line, word emphasis,
// selection) under the syntax colours.
type Span struct {
	// Text is plain text without escape sequences or tabs.
	Text string
	// Fg is the syntax colour from t.Syntax; nil means t.Syntax.Plain.
	Fg color.Color
	// Emph marks a changed word inside an added or removed line (word-level
	// highlight).
	Emph bool
	// Bold and Italic are syntax font styles.
	Bold, Italic bool
}
