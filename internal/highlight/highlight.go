// Package highlight is the neutral syntax-highlighting layer. Callers get
// tokens with neutral kinds and map them to theme colours themselves; the
// chroma implementation lives in a subpackage.
package highlight

// Kind is a neutral token class. Themes map kinds to colours.
type Kind uint8

// Token kinds.
const (
	Plain Kind = iota
	Keyword
	Type
	Function
	String
	Number
	Comment
	Operator
	Punctuation
	Constant
	Variable
	Tag
	Attribute
	Preprocessor
	Error
)

// Token is a run of text with one kind. Tokens never contain newlines.
type Token struct {
	Text string
	Kind Kind
}

// Highlighter tokenises source code.
type Highlighter interface {
	// Lines tokenises src and returns one slice of tokens per line.
	// path picks the language (by extension or name); unknown languages
	// return Plain tokens. Concatenating a line's tokens gives the line.
	Lines(path, src string) [][]Token
	// Language returns the detected language name for path, or "".
	Language(path string) string
}
