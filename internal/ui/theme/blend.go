package theme

import (
	"image/color"

	"github.com/lucasb-eyer/go-colorful"
)

// Blend interpolates from a to b in the perceptual OKLab space. t is clamped to
// [0,1]: 0 returns a, 1 returns b. A nil or fully transparent colour is
// treated as the other endpoint, so blending never yields nil unless both are
// nil. The result is always a plain RGB colour, so only blend for truecolor
// themes (see Theme.Animated).
func Blend(a, b color.Color, t float64) color.Color {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return b
	case b == nil:
		return a
	}
	ca, okA := colorful.MakeColor(a)
	cb, okB := colorful.MakeColor(b)
	switch {
	case !okA && !okB:
		return a
	case !okA:
		return b
	case !okB:
		return a
	}
	if t <= 0 {
		return toRGBA(ca)
	}
	if t >= 1 {
		return toRGBA(cb)
	}
	return toRGBA(ca.BlendOkLab(cb, t).Clamped())
}

func toRGBA(c colorful.Color) color.RGBA {
	r, g, b := c.RGB255()
	return color.RGBA{R: r, G: g, B: b, A: 0xFF}
}
