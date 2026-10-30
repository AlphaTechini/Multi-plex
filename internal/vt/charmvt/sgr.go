package charmvt

import (
	"image/color"
	"strconv"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// appendSGR appends the escape sequence that sets exactly style s, starting
// from a reset. A zero style is the bare reset "\x1b[m".
func appendSGR(b []byte, s *uv.Style) []byte {
	if s.Attrs == 0 && s.Underline == uv.UnderlineNone && s.Fg == nil && s.Bg == nil && s.UnderlineColor == nil {
		return append(b, "\x1b[m"...)
	}
	b = append(b, "\x1b[0"...)
	if s.Attrs != 0 {
		for _, a := range sgrAttrs {
			if s.Attrs&a.bit != 0 {
				b = append(b, ';', a.code)
			}
		}
	}
	switch s.Underline {
	case uv.UnderlineNone:
	case uv.UnderlineSingle:
		b = append(b, ";4"...)
	case uv.UnderlineDouble:
		b = append(b, ";4:2"...)
	case uv.UnderlineCurly:
		b = append(b, ";4:3"...)
	case uv.UnderlineDotted:
		b = append(b, ";4:4"...)
	case uv.UnderlineDashed:
		b = append(b, ";4:5"...)
	default:
		b = append(b, ";4"...)
	}
	b = appendColor(b, s.Fg, 30, 90, 38)
	b = appendColor(b, s.Bg, 40, 100, 48)
	b = appendColor(b, s.UnderlineColor, 0, 0, 58)
	return append(b, 'm')
}

// sgrAttrs maps uv attribute bits to their SGR codes, in output order.
var sgrAttrs = [...]struct {
	bit  uint8
	code byte
}{
	{uv.AttrBold, '1'},
	{uv.AttrFaint, '2'},
	{uv.AttrItalic, '3'},
	{uv.AttrBlink, '5'},
	{uv.AttrRapidBlink, '6'},
	{uv.AttrReverse, '7'},
	{uv.AttrConceal, '8'},
	{uv.AttrStrikethrough, '9'},
}

// appendColor appends the SGR parameters for colour c. base and bright are
// the first codes of the 8 normal and 8 bright colours (0 where there is no
// short form, as for the underline colour); ext is 38, 48 or 58.
func appendColor(b []byte, c color.Color, base, bright, ext int) []byte {
	switch v := c.(type) {
	case nil:
		return b
	case ansi.BasicColor:
		if base != 0 {
			if v < 8 {
				return strconv.AppendInt(append(b, ';'), int64(base)+int64(v), 10)
			}
			return strconv.AppendInt(append(b, ';'), int64(bright)+int64(v)-8, 10)
		}
		return appendIndexed(b, ext, uint8(v))
	case ansi.IndexedColor:
		return appendIndexed(b, ext, uint8(v))
	}
	r, g, bl, _ := c.RGBA()
	b = strconv.AppendInt(append(b, ';'), int64(ext), 10)
	b = append(b, ";2;"...)
	b = strconv.AppendInt(b, int64(r>>8), 10)
	b = append(b, ';')
	b = strconv.AppendInt(b, int64(g>>8), 10)
	b = append(b, ';')
	return strconv.AppendInt(b, int64(bl>>8), 10)
}

func appendIndexed(b []byte, ext int, n uint8) []byte {
	b = strconv.AppendInt(append(b, ';'), int64(ext), 10)
	b = append(b, ";5;"...)
	return strconv.AppendInt(b, int64(n), 10)
}
