package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

var sampleHints = []Hint{{Keys: "ctrl+space p", Label: "palette"}, {Keys: "?", Label: "keys"}}

// each runs fn for every theme variant and width.
func each(t *testing.T, fn func(t *testing.T, th *theme.Theme, w int)) {
	for name, th := range themes() {
		t.Run(name, func(t *testing.T) {
			for _, w := range widths {
				fn(t, th, w)
			}
		})
	}
}

func TestPanel(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for h := 0; h < 7; h++ {
			for _, focused := range []bool{false, true} {
				o := PanelOptions{Width: w, Height: h, Title: "auth-refactor", Meta: "claude", Focused: focused,
					Icon: StateDot(th, StateWorking, 3), Footer: KeyHints(th, sampleHints, 0),
					Content: "line one\n" + styled + "\n" + strings.Repeat("long ", 40)}
				out := Panel(th, o)
				assertBlock(t, "Panel", out, w, h)
				noLeak(t, "Panel", out)
				o.Flush = true
				o.Header = TabBar(th, TabBarOptions{Tabs: []Tab{{Label: "Overview", Badge: "3"}, {Label: "Diff"}}, Active: 0})
				assertBlock(t, "Panel flush", Panel(th, o), w, h)
			}
		}
	})
}

func TestPanelInner(t *testing.T) {
	w, h := PanelInner(PanelOptions{Width: 30, Height: 10})
	if w != 26 || h != 8 {
		t.Errorf("PanelInner = %d×%d, want 26×8", w, h)
	}
	w, _ = PanelInner(PanelOptions{Width: 30, Height: 10, Flush: true})
	if w != 28 {
		t.Errorf("PanelInner flush = %d, want 28", w)
	}
}

func TestTabBar(t *testing.T) {
	tabs := []Tab{{Label: "Overview", Badge: "3"}, {Label: "Diff", Badge: "7", Alert: true}, {Label: "Code"}, {Label: "Specs"}, {Label: "Term"}}
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		if w == 0 {
			return // 0 means natural width, tested below
		}
		for active := range tabs {
			assertWidth(t, "TabBar", TabBar(th, TabBarOptions{Tabs: tabs, Active: active, Width: w}), w)
		}
	})
	th := themes()["truecolor"]
	// Switching tabs must not change the bar's natural width (no jitter).
	w0 := ansi.StringWidth(TabBar(th, TabBarOptions{Tabs: tabs, Active: 0}))
	for a := range tabs {
		if got := ansi.StringWidth(TabBar(th, TabBarOptions{Tabs: tabs, Active: a})); got != w0 {
			t.Errorf("TabBar width changes with the active tab: %d vs %d", got, w0)
		}
	}
}

func TestStateDotAndBadge(t *testing.T) {
	for _, th := range themes() {
		for s := StateIdle; s <= StateError; s++ {
			for frame := -3; frame < 15; frame++ {
				if w := ansi.StringWidth(StateDot(th, s, frame)); w != 1 {
					t.Errorf("StateDot(%v) is %d cells", s, w)
				}
			}
			badge := ansi.Strip(StateBadge(th, s, 0))
			if !strings.HasSuffix(badge, " "+s.String()) {
				t.Errorf("StateBadge(%v) = %q", s, badge)
			}
		}
	}
	reduced := themes()["reduced"]
	if StateDot(reduced, StateWorking, 0) != StateDot(reduced, StateWorking, 5) {
		t.Error("reduced motion must use a static working glyph")
	}
}

func TestDividerSectionRowProgress(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for _, a := range []Align{AlignLeft, AlignCenter, AlignRight} {
			assertWidth(t, "Divider", Divider(th, DividerOptions{Width: w, Label: "end of file", Align: a}), w)
		}
		assertWidth(t, "Divider plain", Divider(th, DividerOptions{Width: w}), w)
		assertWidth(t, "Section", Section(th, SectionOptions{Width: w, Label: "Sessions", Meta: "3"}), w)
		for _, sel := range []bool{false, true} {
			assertWidth(t, "Row", Row(th, RowOptions{Width: w, Left: "token-refresh " + styled, Right: "3/7", Selected: sel, Focused: true}), w)
		}
		assertWidth(t, "Progress", Progress(th, ProgressOptions{Width: w, Done: 3, Total: 7, Label: true}), w)
		assertWidth(t, "Progress no total", Progress(th, ProgressOptions{Width: w}), w)
		if w > 0 { // 0 means natural width
			assertWidth(t, "KeyHints", KeyHints(th, sampleHints, w), w)
			assertWidth(t, "Choice", Choice(th, ChoiceOptions{Width: w, Options: []string{"implementer", "planner"}, Selected: 1, Focused: true}), w)
		}
	})
}

func TestSessionItem(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for s := StateIdle; s <= StateError; s++ {
			out := SessionItem(th, SessionItemOptions{Width: w, Name: "api-pagination", State: s, Detail: "needs approval",
				Badge: "12", Selected: s == StateBlocked, Focused: true, Unseen: s == StateDone})
			assertBlock(t, "SessionItem", out, w, 2)
		}
	})
}

func TestCard(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for sev := SeverityInfo; sev <= SeverityDecision; sev++ {
			for _, sel := range []bool{false, true} {
				o := CardOptions{Width: w, Number: 12, Severity: sev, Title: "Changing Session.Refresh signature",
					Body: strings.Repeat("Callers in api/ and worker/ will need updating. ", 6), Refs: []string{"src/auth/session.go:L40-L52"},
					Age: "2m", Actions: []Hint{{Keys: "a", Label: "ask"}, {Keys: "s", Label: "steer"}}, Selected: sel}
				assertBlock(t, "Card", Card(th, o), w, CardHeight(o))
			}
		}
	})
	if h := CardHeight(CardOptions{}); h != 3 {
		t.Errorf("CardHeight of an empty card = %d, want 3", h)
	}
}

func TestBannerAndFileEnd(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for _, settle := range []float64{0, 0.4, 1} {
			assertWidth(t, "Banner", Banner(th, BannerOptions{Width: w, Path: "src/auth/session.go", Index: 3, Total: 7,
				Status: "modified", Added: 24, Removed: 9, Settle: settle}), w)
		}
		assertWidth(t, "FileEnd", FileEnd(th, FileEndOptions{Width: w, Next: "src/auth/session_test.go", Keys: "]"}), w)
		assertWidth(t, "FileEnd last", FileEnd(th, FileEndOptions{Width: w, Total: 7}), w)
	})
}

func TestDiffLine(t *testing.T) {
	spans := []Span{{Text: "\ttok, err := "}, {Text: "s.cache.Get(ctx)", Emph: true}, {Text: " // 世界", Italic: true}}
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for k := DiffContext; k <= DiffFold; k++ {
			for _, sel := range []bool{false, true} {
				o := DiffLineOptions{Width: w, Kind: k, Old: 44, New: 1044, Spans: spans, Header: "@@ -38,9 +38,14 @@",
					Folded: 42, Selected: sel, Highlight: 0.5}
				assertWidth(t, "DiffLine", DiffLine(th, o), w)
			}
		}
	})
	if DiffGutterWidth(4) != 13 || DiffGutterWidth(0) != 13 {
		t.Error("DiffGutterWidth")
	}
}

func TestCommentBoxFieldToast(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for _, focused := range []bool{false, true} {
			cb := CommentBox(th, CommentBoxOptions{Width: w, Indent: 11, Author: "You", Status: "draft", Range: "L44–L46",
				Body: strings.Repeat("cache the token instead ", 5), Hints: sampleHints, Focused: focused})
			if w > 0 {
				assertWidth(t, "CommentBox", cb, w)
			}
			f := FieldOptions{Width: w, Label: "Task", Help: "ctrl+t", Value: strings.Repeat("make it per user ", 6), Rows: 3, Focused: focused, Cursor: true}
			assertBlock(t, "Field", Field(th, f), w, FieldHeight(f))
			f.Value = ""
			f.Placeholder = "Describe the task"
			assertBlock(t, "Field empty", Field(th, f), w, FieldHeight(f))
		}
		for _, tone := range []Tone{ToneSuccess, ToneWarning, ToneDanger, ToneInfo} {
			out := Toast(th, ToastOptions{Width: w, Tone: tone, Title: "3 comments sent", Body: "to auth-refactor", Hints: sampleHints})
			if w > 0 {
				assertBlock(t, "Toast", out, w, 5)
			}
		}
	})
}

func TestStatusBar(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for _, withRight := range []bool{false, true} {
			o := StatusBarOptions{Width: w,
				Left:  []Segment{{Text: "multi-plex", Bold: true}, {Text: "auth-refactor"}, {Text: StateBadge(th, StateWorking, 1)}},
				Hints: sampleHints}
			if withRight {
				o.Right = []Segment{{Text: "1 approval", Tone: ToneWarning, Pill: true}}
			}
			assertWidth(t, "StatusBar", StatusBar(th, o), w)
		}
	})
	th := themes()["truecolor"]
	// The approval pill outranks trailing left segments at narrow widths.
	out := ansi.Strip(StatusBar(th, StatusBarOptions{Width: 50,
		Left:  []Segment{{Text: "multi-plex"}, {Text: "auth-refactor"}, {Text: "mpx/auth-refactor-long-branch"}},
		Right: []Segment{{Text: "1 approval", Pill: true}}, Hints: sampleHints}))
	if !strings.Contains(out, "1 approval") || !strings.Contains(out, "multi-plex") {
		t.Errorf("StatusBar dropped the wrong segment: %q", out)
	}
}

func TestEmptyState(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		for h := 0; h < 12; h += 3 {
			out := EmptyState(th, EmptyStateOptions{Width: w, Height: h, Eyebrow: "multiplex", Title: "No sessions yet",
				Body: "A session is one agent working on one task.", Hints: sampleHints})
			assertBlock(t, "EmptyState", out, w, h)
		}
	})
}

func TestModal(t *testing.T) {
	each(t, func(t *testing.T, th *theme.Theme, w int) {
		screen := FitBlock("background "+styled, w, 20)
		for _, flush := range []bool{false, true} {
			o := ModalOptions{Width: 60, Title: "Approve?", Meta: "api-pagination", Tone: ToneWarning, Flush: flush,
				Body: "rm -rf testdata\n" + strings.Repeat("x", 90), Buttons: []ButtonOptions{{Label: "Approve once", Keys: "enter", Variant: ButtonPrimary}, {Label: "Deny", Keys: "esc"}},
				Hints: sampleHints, Screen: screen, ScreenWidth: w, ScreenHeight: 20, Top: -1}
			out := Modal(th, o)
			if w > 0 {
				assertBlock(t, "Modal", out, w, 20)
			}
			o.Screen = ""
			box := Modal(th, o)
			if bw := min(60, w-4); bw >= 6 {
				assertWidth(t, "Modal box", box, bw)
			}
		}
	})
}

func TestButtons(t *testing.T) {
	th := themes()["truecolor"]
	for _, v := range []ButtonVariant{ButtonPrimary, ButtonSecondary, ButtonGhost} {
		got := ansi.Strip(Button(th, ButtonOptions{Label: "Deny", Keys: "esc", Variant: v}))
		if got != " Deny  esc " {
			t.Errorf("Button(%v) = %q", v, got)
		}
	}
	row := ButtonRow(th, []ButtonOptions{{Label: "A"}, {Label: "B"}, {Label: "C"}}, 10)
	if ansi.StringWidth(row) > 10 {
		t.Errorf("ButtonRow exceeded width: %d", ansi.StringWidth(row))
	}
	if got := ansi.Strip(Keycap(th, "n")); got != " n " {
		t.Errorf("Keycap = %q", got)
	}
}
