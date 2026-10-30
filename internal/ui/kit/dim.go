package kit

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Dim blends every foreground and background colour in s toward the colour
// toward by amount (0 = unchanged, 1 = fully toward). Text with the terminal's
// default colours is treated as t.Text.Primary on t.Surface.Base. Use it for
// the scrim behind modals (toward t.Surface.Sunken, amount about 0.6) and for
// the fade-in after a file switch (toward t.Surface.Base, amount falling from
// 1 to 0). The result uses truecolor sequences; on 256-colour terminals the
// writer converts them, so only animate when t.Animated() is true.
func Dim(t *theme.Theme, s string, toward color.Color, amount float64) string {
	if amount <= 0 || t == nil {
		return s
	}
	amount = min(amount, 1)
	d := dimmer{toward: toward, amount: amount, cache: map[string]string{}}
	defFg := d.blend(t.Text.Primary)
	defBg := d.blend(t.Surface.Base)
	head := "\x1b[" + truecolorParam(38, defFg) + ";" + truecolorParam(48, defBg) + "m"
	var b strings.Builder
	b.Grow(len(s) + len(s)/4 + 64)
	b.WriteString(head)
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '\n':
			b.WriteString(ansi.ResetStyle)
			b.WriteByte('\n')
			b.WriteString(head)
			i++
		case c == 0x1B && i+1 < len(s) && s[i+1] == '[':
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7E) && s[j] != '\n' {
				j++
			}
			switch {
			case j >= len(s) || s[j] == '\n':
				b.WriteString(s[i:j]) // unterminated: copy as it is
				i = j
				continue
			case s[j] == 'm':
				b.WriteString("\x1b[")
				b.WriteString(d.rewrite(s[i+2:j], defFg, defBg))
				b.WriteByte('m')
			default:
				b.WriteString(s[i : j+1])
			}
			i = j + 1
		default:
			k := i + 1
			for k < len(s) && s[k] != '\n' && s[k] != 0x1B {
				k++
			}
			b.WriteString(s[i:k])
			i = k
		}
	}
	b.WriteString(ansi.ResetStyle)
	return b.String()
}

type dimmer struct {
	toward color.Color
	amount float64
	cache  map[string]string
}

func (d dimmer) blend(c color.Color) color.Color {
	return theme.Blend(c, d.toward, d.amount)
}

// rewrite returns the SGR parameters p with every colour blended.
func (d dimmer) rewrite(p string, defFg, defBg color.Color) string {
	if out, ok := d.cache[p]; ok {
		return out
	}
	parts := sgrParams(p)
	out := make([]string, 0, len(parts)+4)
	for i := 0; i < len(parts); i++ {
		v := parts[i]
		n, err := strconv.Atoi(v)
		if err != nil {
			out = append(out, v)
			continue
		}
		switch {
		case n == 0:
			out = append(out, "0", truecolorParam(38, defFg), truecolorParam(48, defBg))
		case n == 39:
			out = append(out, truecolorParam(38, defFg))
		case n == 49:
			out = append(out, truecolorParam(48, defBg))
		case n >= 30 && n <= 37, n >= 90 && n <= 97:
			out = append(out, truecolorParam(38, d.blend(basic(n, 30, 90))))
		case n >= 40 && n <= 47, n >= 100 && n <= 107:
			out = append(out, truecolorParam(48, d.blend(basic(n, 40, 100))))
		case n == 38 || n == 48:
			args := colorArgs(parts, i)
			c := extended(parts[i+1 : i+1+args])
			if c == nil {
				out = append(out, parts[i:i+1+args]...)
			} else {
				out = append(out, truecolorParam(n, d.blend(c)))
			}
			i += args
		case n == 58:
			args := colorArgs(parts, i)
			out = append(out, parts[i:i+1+args]...)
			i += args
		default:
			out = append(out, v)
		}
	}
	res := strings.Join(out, ";")
	d.cache[p] = res
	return res
}

// basic converts an SGR basic colour code to its colour.
func basic(n, normal, bright int) color.Color {
	if n >= bright {
		return ansi.BasicColor(n - bright + 8)
	}
	return ansi.BasicColor(n - normal)
}

// extended parses the arguments of a 38/48 colour ("5;n" or "2;r;g;b").
func extended(args []string) color.Color {
	nums := make([]uint8, 0, len(args))
	for _, a := range args {
		v, err := strconv.Atoi(a)
		if err != nil || v < 0 || v > 255 {
			return nil
		}
		nums = append(nums, uint8(v))
	}
	switch {
	case len(nums) == 2 && nums[0] == 5:
		return ansi.IndexedColor(nums[1])
	case len(nums) == 4 && nums[0] == 2:
		return color.RGBA{R: nums[1], G: nums[2], B: nums[3], A: 0xFF}
	}
	return nil
}

// truecolorParam formats "38;2;r;g;b" (or 48) for c.
func truecolorParam(intro int, c color.Color) string {
	if c == nil {
		return strconv.Itoa(intro + 1)
	}
	r, g, b, _ := c.RGBA()
	buf := make([]byte, 0, 20)
	buf = strconv.AppendInt(buf, int64(intro), 10)
	buf = append(buf, ";2;"...)
	buf = strconv.AppendUint(buf, uint64(r>>8), 10)
	buf = append(buf, ';')
	buf = strconv.AppendUint(buf, uint64(g>>8), 10)
	buf = append(buf, ';')
	buf = strconv.AppendUint(buf, uint64(b>>8), 10)
	return string(buf)
}
