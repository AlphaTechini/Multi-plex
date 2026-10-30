package diffview

import (
	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/ui/theme"

	"image/color"
)

// controlWords are the keywords Dark+ colours as control flow (purple); the
// other keywords use the declaration colour (blue).
var controlWords = map[string]bool{
	"if": true, "else": true, "elif": true, "for": true, "while": true, "do": true,
	"return": true, "break": true, "continue": true, "switch": true, "case": true,
	"default": true, "defer": true, "go": true, "select": true, "range": true,
	"goto": true, "try": true, "catch": true, "finally": true, "throw": true,
	"raise": true, "except": true, "yield": true, "await": true, "import": true,
	"from": true, "package": true, "match": true, "when": true, "fallthrough": true,
	"export": true, "with": true, "assert": true, "pass": true, "in": true,
}

// keywordConstants are literal constants Dark+ colours like keywords.
var keywordConstants = map[string]bool{
	"nil": true, "true": true, "false": true, "null": true, "None": true,
	"True": true, "False": true, "undefined": true, "this": true, "self": true,
	"iota": true, "NULL": true,
}

// syntaxStyle maps a highlight token to its theme colour. It is the one place
// where highlight kinds meet theme.Syntax.
func syntaxStyle(t *theme.Theme, tok highlight.Token) color.Color {
	s := &t.Syntax
	switch tok.Kind {
	case highlight.Keyword:
		if controlWords[tok.Text] {
			return s.Control
		}
		return s.Keyword
	case highlight.Constant:
		if keywordConstants[tok.Text] {
			return s.Keyword
		}
		return s.Constant
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
	case highlight.Variable, highlight.Attribute:
		return s.Variable
	case highlight.Tag:
		return s.Keyword
	case highlight.Preprocessor:
		return s.Control
	}
	return s.Plain
}
