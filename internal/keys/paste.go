package keys

import "strings"

const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// EncodePaste returns the bytes to send for pasted text. With bracketed paste
// (the program enabled mode 2004) the text is wrapped in the paste markers
// and sent as is, except that an embedded end marker is removed, since it
// would let the pasted text escape the bracket and run as typed input.
// Without it, newlines become carriage returns as in a terminal, and control
// characters other than Tab and Enter are dropped.
func EncodePaste(text string, bracketed bool) []byte {
	if bracketed {
		for strings.Contains(text, pasteEnd) {
			text = strings.ReplaceAll(text, pasteEnd, "")
		}
		b := make([]byte, 0, len(pasteStart)+len(text)+len(pasteEnd))
		b = append(b, pasteStart...)
		b = append(b, text...)
		return append(b, pasteEnd...)
	}
	b := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '\r':
			b = append(b, '\r')
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
		case c == '\n':
			b = append(b, '\r')
		case c == '\t' || c >= 0x20 && c != 0x7f:
			b = append(b, c)
		}
	}
	return b
}
