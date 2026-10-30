// Package syntax maps neutral highlight kinds to theme colours and provides
// an asynchronous, windowed, cached highlighter for the Code view. Chroma
// costs about 100 µs per line, so a view never tokenises a whole file on the
// render path: it asks a Source for the lines it shows and draws plain text
// until the chunk containing them arrives.
package syntax

import (
	"image/color"

	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// control lists the keywords Dark+ draws as control flow (purple) rather than
// as declarations (blue). It is language-agnostic on purpose: the neutral
// highlight layer only knows "keyword".
var control = map[string]struct{}{
	"if": {}, "else": {}, "elif": {}, "elsif": {}, "for": {}, "foreach": {}, "while": {}, "do": {},
	"switch": {}, "case": {}, "default": {}, "break": {}, "continue": {}, "return": {}, "goto": {},
	"defer": {}, "go": {}, "select": {}, "try": {}, "catch": {}, "except": {}, "finally": {},
	"throw": {}, "raise": {}, "yield": {}, "await": {}, "import": {}, "from": {}, "export": {},
	"with": {}, "assert": {}, "range": {}, "match": {}, "when": {}, "unless": {}, "until": {},
}

// Color returns the theme colour for a token of kind k whose text is text.
// The text only matters for keywords, which split into control flow and
// declarations.
func Color(t *theme.Theme, k highlight.Kind, text string) color.Color {
	s := &t.Syntax
	switch k {
	case highlight.Keyword:
		if _, ok := control[text]; ok {
			return s.Control
		}
		return s.Keyword
	case highlight.Tag:
		return s.Keyword
	case highlight.Preprocessor:
		return s.Keyword
	case highlight.Type:
		return s.Type
	case highlight.Function:
		return s.Function
	case highlight.String:
		return s.String
	case highlight.Number:
		return s.Number
	case highlight.Comment:
		return s.Comment
	case highlight.Constant:
		return s.Constant
	case highlight.Variable, highlight.Attribute:
		return s.Variable
	default:
		return s.Plain
	}
}
