package diff

import "strings"

// unquoteC decodes a git C-style quoted string at the start of s (s[0] must
// be '"'). It returns the decoded value and the text after the closing quote.
// git quotes paths containing control characters, quotes, backslashes or (by
// default) non-ASCII bytes, writing the latter as three-digit octal escapes.
func unquoteC(s string) (val, rest string, ok bool) {
	if len(s) == 0 || s[0] != '"' {
		return "", s, false
	}
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			return b.String(), s[i+1:], true
		case c != '\\':
			b.WriteByte(c)
		default:
			i++
			if i >= len(s) {
				return "", s, false
			}
			switch e := s[i]; e {
			case 'a':
				b.WriteByte('\a')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'v':
				b.WriteByte('\v')
			case '"', '\\':
				b.WriteByte(e)
			case '0', '1', '2', '3', '4', '5', '6', '7':
				v := int(e - '0')
				for n := 0; n < 2 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7'; n++ {
					i++
					v = v*8 + int(s[i]-'0')
				}
				b.WriteByte(byte(v))
			default:
				// Unknown escape: keep it verbatim rather than fail.
				b.WriteByte('\\')
				b.WriteByte(e)
			}
		}
	}
	return "", s, false
}
