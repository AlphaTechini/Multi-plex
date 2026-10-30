package chromahl

import (
	"github.com/alecthomas/chroma/v2"

	"github.com/alphatechini/multi-plex/internal/highlight"
)

// kindOf maps a chroma token type to the neutral kind. The choices follow the
// VS Code Dark+ theme: built-in functions share the function colour, built-in
// and keyword types share the type colour, word operators (and, or, not)
// are keywords.
func kindOf(t chroma.TokenType) highlight.Kind {
	switch t {
	case chroma.Error:
		return highlight.Error
	case chroma.KeywordConstant, chroma.NameConstant:
		return highlight.Constant
	case chroma.KeywordType, chroma.NameClass, chroma.NameException, chroma.NameNamespace:
		return highlight.Type
	case chroma.NameBuiltin, chroma.NameDecorator:
		return highlight.Function
	case chroma.NameBuiltinPseudo, chroma.NameKeyword, chroma.OperatorWord, chroma.OperatorReserved:
		return highlight.Keyword
	case chroma.NameTag:
		return highlight.Tag
	case chroma.NameAttribute:
		return highlight.Attribute
	case chroma.NameProperty, chroma.NameOther, chroma.NamePseudo:
		return highlight.Variable
	case chroma.NameOperator:
		return highlight.Operator
	case chroma.CommentPreproc, chroma.CommentPreprocFile:
		return highlight.Preprocessor
	case chroma.LiteralDate:
		return highlight.Number
	case chroma.GenericHeading, chroma.GenericSubheading:
		return highlight.Keyword
	}
	switch t.Category() {
	case chroma.Keyword:
		return highlight.Keyword
	case chroma.Name:
		switch t.SubCategory() {
		case chroma.NameVariable:
			return highlight.Variable
		case chroma.NameFunction:
			return highlight.Function
		}
		return highlight.Plain
	case chroma.Literal:
		switch t.SubCategory() {
		case chroma.LiteralString:
			return highlight.String
		case chroma.LiteralNumber:
			return highlight.Number
		}
		return highlight.Plain
	case chroma.Operator:
		return highlight.Operator
	case chroma.Punctuation:
		return highlight.Punctuation
	case chroma.Comment:
		return highlight.Comment
	}
	return highlight.Plain
}
