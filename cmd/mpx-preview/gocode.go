package main

import (
	"image/color"
	"strings"
	"unicode"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

var goKeywords = map[string]bool{
	"func": true, "var": true, "const": true, "type": true, "struct": true, "interface": true,
	"package": true, "import": true, "map": true, "chan": true, "nil": true, "true": true, "false": true,
}

var goControl = map[string]bool{
	"if": true, "else": true, "return": true, "for": true, "range": true, "defer": true,
	"go": true, "switch": true, "case": true, "break": true, "continue": true,
}

var goTypes = map[string]bool{
	"error": true, "string": true, "int": true, "bool": true, "any": true, "Context": true,
	"Session": true, "Time": true, "Duration": true, "Token": true,
}

// goSpans is a toy Go highlighter for the preview. Text between « and » is
// marked as a changed word (word-level emphasis); the markers are removed.
func goSpans(t *theme.Theme, line string) []kit.Span {
	var spans []kit.Span
	emph := false
	push := func(text string, c color.Color) {
		if text != "" {
			spans = append(spans, kit.Span{Text: text, Fg: c, Emph: emph})
		}
	}
	rs := []rune(line)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '«':
			emph = true
			i++
		case r == '»':
			emph = false
			i++
		case r == '/' && i+1 < len(rs) && rs[i+1] == '/':
			push(string(rs[i:]), t.Syntax.Comment)
			i = len(rs)
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			j = min(j+1, len(rs))
			push(string(rs[i:j]), t.Syntax.String)
			i = j
		case unicode.IsDigit(r):
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.') {
				j++
			}
			push(string(rs[i:j]), t.Syntax.Number)
			i = j
		case unicode.IsLetter(r) || r == '_':
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
				j++
			}
			word := string(rs[i:j])
			c := t.Syntax.Variable
			switch {
			case goKeywords[word]:
				c = t.Syntax.Keyword
			case goControl[word]:
				c = t.Syntax.Control
			case goTypes[word]:
				c = t.Syntax.Type
			case j < len(rs) && rs[j] == '(':
				c = t.Syntax.Function
			case i > 0 && rs[i-1] == '.' && unicode.IsUpper(r):
				c = t.Syntax.Variable
			case unicode.IsUpper(r) && strings.ToUpper(word) == word && len(word) > 1:
				c = t.Syntax.Constant
			}
			push(word, c)
			i = j
		default:
			j := i
			for j < len(rs) && !strings.ContainsRune("«»\"/_", rs[j]) && !unicode.IsLetter(rs[j]) && !unicode.IsDigit(rs[j]) {
				j++
			}
			if j == i {
				j++
			}
			push(string(rs[i:j]), t.Syntax.Plain)
			i = j
		}
	}
	return spans
}
