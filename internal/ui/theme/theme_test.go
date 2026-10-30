package theme

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// tokenGroups returns every colour token of t with its field path.
func tokenGroups(t *Theme) map[string]any {
	out := map[string]any{}
	v := reflect.ValueOf(t).Elem()
	for _, group := range []string{"Surface", "Border", "Text", "Accent", "State", "Severity", "Diff", "Syntax"} {
		g := v.FieldByName(group)
		for i := range g.NumField() {
			out[group+"."+g.Type().Field(i).Name] = g.Field(i).Interface()
		}
	}
	return out
}

func TestEveryTokenSet(t *testing.T) {
	for _, name := range Names() {
		base, ok := ByName(name)
		if !ok {
			t.Fatalf("ByName(%q) not found", name)
		}
		for _, p := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI} {
			th := base.For(p)
			if th.Profile != p {
				t.Errorf("%s.For(%v).Profile = %v", name, p, th.Profile)
			}
			for path, tok := range tokenGroups(th) {
				c, ok := tok.(color.Color)
				if !ok || c == nil {
					t.Errorf("%s/%v: %s is nil", name, p, path)
				}
			}
		}
	}
}

func Test256ProfileUsesExplicitIndexes(t *testing.T) {
	th := DarkPlus().For(colorprofile.ANSI256)
	for path, tok := range tokenGroups(th) {
		if _, ok := tok.(ansi.IndexedColor); !ok {
			t.Errorf("%s is %T, want ansi.IndexedColor", path, tok)
		}
	}
	if got := th.Surface.Base.(ansi.IndexedColor); got != 234 {
		t.Errorf("Surface.Base index = %d, want 234", got)
	}
}

func TestTruecolorHex(t *testing.T) {
	th := DarkPlus()
	if got := Hex(th.Surface.Base); got != "#1E1E1E" {
		t.Errorf("Surface.Base = %s, want Dark+ editor background #1E1E1E", got)
	}
	if got := Hex(th.Syntax.Keyword); got != "#569CD6" {
		t.Errorf("Syntax.Keyword = %s, want #569CD6", got)
	}
}

func TestRegistry(t *testing.T) {
	if _, ok := ByName("no-such-theme"); ok {
		t.Error("ByName found a theme that does not exist")
	}
	th, ok := ByName(DarkPlusName)
	if !ok || th.Name != DarkPlusName {
		t.Fatalf("ByName(%q) = %v, %v", DarkPlusName, th, ok)
	}
	Register("test-copy", func(p colorprofile.Profile) *Theme {
		c := darkPlus(p)
		c.Name = "test-copy"
		return c
	})
	got, ok := ByName("test-copy")
	if !ok || got.Name != "test-copy" {
		t.Fatalf("registered theme not found")
	}
	if got.For(colorprofile.ANSI256).Name != "test-copy" {
		t.Error("For lost the theme's builder")
	}
	found := false
	for _, n := range Names() {
		found = found || n == "test-copy"
	}
	if !found {
		t.Error("Names does not list the registered theme")
	}
}

func TestBlend(t *testing.T) {
	a := color.RGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	b := color.RGBA{R: 0x37, G: 0x94, B: 0xFF, A: 0xFF}
	if got := Hex(Blend(a, b, 0)); got != "#1E1E1E" {
		t.Errorf("Blend(a,b,0) = %s, want a", got)
	}
	if got := Hex(Blend(a, b, 1)); got != "#3794FF" {
		t.Errorf("Blend(a,b,1) = %s, want b", got)
	}
	if got := Hex(Blend(a, b, -3)); got != "#1E1E1E" {
		t.Errorf("Blend clamps t below 0, got %s", got)
	}
	if got := Hex(Blend(a, b, 7)); got != "#3794FF" {
		t.Errorf("Blend clamps t above 1, got %s", got)
	}
	mid := Blend(a, b, 0.5)
	r, g, bb, _ := mid.RGBA()
	if r>>8 <= 0x1E || bb>>8 <= 0x1E || g>>8 <= 0x1E || bb>>8 >= 0xFF {
		t.Errorf("Blend midpoint %s is not between the endpoints", Hex(mid))
	}
	if Blend(nil, b, 0.5) != b || Blend(a, nil, 0.5) != a || Blend(nil, nil, 0.5) != nil {
		t.Error("Blend nil handling")
	}
}

func TestEaseOutCubic(t *testing.T) {
	if EaseOutCubic(0) != 0 || EaseOutCubic(1) != 1 || EaseOutCubic(-1) != 0 || EaseOutCubic(2) != 1 {
		t.Error("EaseOutCubic endpoints or clamping wrong")
	}
	prev := 0.0
	for i := 1; i <= 10; i++ {
		v := EaseOutCubic(float64(i) / 10)
		if v < prev {
			t.Errorf("EaseOutCubic not monotonic at %d", i)
		}
		prev = v
	}
	if EaseOutCubic(0.5) <= 0.5 {
		t.Error("EaseOutCubic should be ahead of linear at the midpoint")
	}
}

func TestGlyphsAreOneCell(t *testing.T) {
	for name, g := range map[string]Glyphs{"unicode": UnicodeGlyphs(), "ascii": ASCIIGlyphs()} {
		v := reflect.ValueOf(g)
		for i := range v.NumField() {
			f := v.Type().Field(i)
			switch val := v.Field(i).Interface().(type) {
			case string:
				// Key names may be words ("esc", "tab"); every symbol is one cell.
				if isKeyName(f.Name) {
					continue
				}
				if w := ansi.StringWidth(val); w != 1 {
					t.Errorf("%s.%s = %q is %d cells, want 1", name, f.Name, val, w)
				}
			case []string:
				if len(val) == 0 {
					t.Errorf("%s.%s is empty", name, f.Name)
				}
				for _, s := range val {
					if ansi.StringWidth(s) != 1 {
						t.Errorf("%s.%s frame %q is not one cell", name, f.Name, s)
					}
				}
			case Box:
				for _, s := range []string{val.TopLeft, val.TopRight, val.BottomLeft, val.BottomRight, val.Horizontal, val.Vertical} {
					if ansi.StringWidth(s) != 1 {
						t.Errorf("%s.Frame part %q is not one cell", name, s)
					}
				}
			}
		}
	}
}

func isKeyName(field string) bool {
	switch field {
	case "Ctrl", "Shift", "Alt", "Enter", "Esc", "Space", "Tab", "Backspace":
		return true
	}
	return false
}

func TestASCIIGlyphsArePureASCII(t *testing.T) {
	g := ASCIIGlyphs()
	v := reflect.ValueOf(g)
	check := func(field, s string) {
		for _, r := range s {
			if r > 0x7E {
				t.Errorf("ASCIIGlyphs.%s = %q contains non-ASCII %q", field, s, r)
			}
		}
	}
	for i := range v.NumField() {
		switch val := v.Field(i).Interface().(type) {
		case string:
			check(v.Type().Field(i).Name, val)
		case []string:
			for _, s := range val {
				check(v.Type().Field(i).Name, s)
			}
		}
	}
}

func TestWithCopies(t *testing.T) {
	base := DarkPlus()
	a := base.WithGlyphs(ASCIIGlyphs())
	if base.Glyphs.Unicode == false || a.Glyphs.Unicode {
		t.Error("WithGlyphs must not modify the receiver")
	}
	r := base.WithReducedMotion(true)
	if base.Motion.Reduce || !r.Motion.Reduce || r.Animated() {
		t.Error("WithReducedMotion wrong")
	}
	if !base.Animated() || base.For(colorprofile.ANSI256).Animated() {
		t.Error("Animated should be true only for truecolor without reduced motion")
	}
	if a.For(colorprofile.ANSI256).Glyphs.Unicode {
		t.Error("For must keep the glyph set")
	}
}
