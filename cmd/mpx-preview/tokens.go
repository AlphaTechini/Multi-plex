package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// tokens prints the theme's colour tokens as a Markdown table (token, hex,
// 256-colour index) with a swatch, for docs/16 and for checking a new theme.
func (s scene) tokens() string {
	tc := s.t.For(colorprofile.TrueColor)
	x256 := s.t.For(colorprofile.ANSI256)
	var b strings.Builder
	b.WriteString("| Token | Hex | 256 |\n|---|---|---|\n")
	tv, iv := reflect.ValueOf(tc).Elem(), reflect.ValueOf(x256).Elem()
	for _, group := range []string{"Surface", "Border", "Text", "Accent", "State", "Severity", "Diff", "Syntax"} {
		g, gi := tv.FieldByName(group), iv.FieldByName(group)
		for i := range g.NumField() {
			name := group + "." + g.Type().Field(i).Name
			hex := theme.Hex(g.Field(i).Interface().(ansi.Color))
			idx := ""
			if c, ok := gi.Field(i).Interface().(ansi.IndexedColor); ok {
				idx = fmt.Sprint(int(c))
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", name, hex, idx)
		}
	}
	return b.String()
}
