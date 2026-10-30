package theme

import (
	"fmt"
	"image/color"
)

// Hex formats a colour as "#RRGGBB" for docs, logs and tests. It returns ""
// for nil.
func Hex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}
