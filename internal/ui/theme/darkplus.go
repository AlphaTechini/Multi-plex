package theme

import "github.com/charmbracelet/colorprofile"

// DarkPlusName is the registry name of the default theme.
const DarkPlusName = "dark-plus"

func init() { Register(DarkPlusName, darkPlus) }

// DarkPlus returns the default theme, resolved for truecolor: VS Code's Dark+
// editor, token and diff colours, refined into a UI palette (docs/16). Call
// For on the result to resolve it for the terminal's real colour profile.
func DarkPlus() *Theme {
	t := darkPlus(colorprofile.TrueColor)
	t.build = darkPlus
	return t
}

// darkPlus builds Dark+ for a profile. Every entry is (truecolor hex,
// 256-colour index). The table in docs/16-visual-design.md mirrors it.
//
// The 256-colour diff is deliberately different: the cube has no quiet dark
// greens or reds (22 and 52 are loud as full-line backgrounds), so added and
// removed lines keep the base background and are carried by the coloured
// gutter, the sign and the word-level emphasis.
func darkPlus(p colorprofile.Profile) *Theme {
	c := NewPicker(p)
	t := &Theme{
		Name:    DarkPlusName,
		Profile: p,
		Surface: Surfaces{
			Sunken:        c("#181818", 233),
			Base:          c("#1E1E1E", 234),
			Raised:        c("#252526", 235),
			Overlay:       c("#2A2A2C", 236),
			Control:       c("#3A3D41", 238),
			Hover:         c("#2A2D2E", 236),
			Selected:      c("#04395E", 24),
			TextSelection: c("#264F78", 24),
		},
		Border: Borders{
			Subtle:  c("#2D2D2D", 235),
			Default: c("#444444", 238),
			Focus:   c("#3794FF", 33),
		},
		Text: Texts{
			Strong:    c("#F0F0F0", 255),
			Primary:   c("#D4D4D4", 252),
			Secondary: c("#A9A9A9", 248),
			Muted:     c("#858585", 244),
			Faint:     c("#5A5A5A", 240),
			OnAccent:  c("#FFFFFF", 231),
			OnState:   c("#1A1A1A", 234),
		},
		Accent: Accents{
			Base:   c("#3794FF", 33),
			Fill:   c("#0E639C", 25),
			Subtle: c("#1D3349", 236),
		},
		State: States{
			Starting: c("#9D9D9D", 247),
			Working:  c("#4FC1FF", 75),
			Blocked:  c("#E8AB53", 215),
			Done:     c("#73C991", 114),
			Idle:     c("#6E6E6E", 242),
			Error:    c("#F14C4C", 203),
		},
		Severity: Severities{
			Info:     c("#3794FF", 33),
			Warning:  c("#E8AB53", 215),
			Decision: c("#C586C0", 176),
		},
		Diff: DiffColors{
			AddLineBg:   c("#1C2E22", 234),
			AddWordBg:   c("#2B5937", 22),
			AddGutterBg: c("#1A291E", 22),
			AddSign:     c("#73C991", 114),
			DelLineBg:   c("#35201F", 234),
			DelWordBg:   c("#6A2C2A", 52),
			DelGutterBg: c("#2F1E1D", 52),
			DelSign:     c("#F48771", 210),
			HunkBg:      c("#1C2530", 235),
			HunkFg:      c("#6F9FCF", 68),
			LineNumber:  c("#6E6E6E", 242),
			FoldFg:      c("#707070", 242),
			NewHunkBg:   c("#24466B", 24),
			Renamed:     c("#C586C0", 176),
			Modified:    c("#E2C08D", 180),
		},
		Syntax: SyntaxColors{
			Plain:    c("#D4D4D4", 252),
			Keyword:  c("#569CD6", 74),
			Control:  c("#C586C0", 175),
			String:   c("#CE9178", 173),
			Number:   c("#B5CEA8", 151),
			Comment:  c("#6A9955", 65),
			Function: c("#DCDCAA", 187),
			Type:     c("#4EC9B0", 79),
			Variable: c("#9CDCFE", 117),
			Constant: c("#4FC1FF", 75),
			Regexp:   c("#D16969", 167),
		},
		Glyphs: UnicodeGlyphs(),
		Motion: DefaultMotion(),
	}
	t.Styles = NewStyles(t)
	return t
}
