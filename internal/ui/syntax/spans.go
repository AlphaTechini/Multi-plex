package syntax

import (
	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Spans converts a line's tokens to kit spans coloured from t. Tabs stay in
// the text; kit expands them when painting.
func Spans(t *theme.Theme, toks []highlight.Token) []kit.Span {
	out := make([]kit.Span, len(toks))
	for i, tok := range toks {
		out[i] = kit.Span{Text: tok.Text, Fg: Color(t, tok.Kind, tok.Text)}
	}
	return out
}
