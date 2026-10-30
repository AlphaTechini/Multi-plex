package kit

import (
	"strings"
	"testing"
)

// benchContent is a realistic panel body: styled text, box drawing, glyphs.
func benchContent(w, h int) string {
	th := themes()["truecolor"]
	lines := make([]string, h)
	for i := range lines {
		switch i % 4 {
		case 0:
			lines[i] = SessionItem(th, SessionItemOptions{Width: w, Name: "auth-refactor", State: StateWorking, Detail: "implementer · 14m", Badge: "3"})
			lines[i] = strings.Split(lines[i], "\n")[0]
		case 1:
			lines[i] = "  " + fg(th.Text.Secondary).paint("Refresh now takes a cache argument. Callers in api/ and worker/ need updating;")
		case 2:
			lines[i] = Divider(th, DividerOptions{Width: w - 4, Label: "end of file"})
		default:
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

func BenchmarkPanel92x47(b *testing.B) {
	th := themes()["truecolor"]
	o := PanelOptions{Width: 92, Height: 47, Title: "auth-refactor", Meta: "claude", Focused: true, Flush: true}
	iw, ih := PanelInner(o)
	o.Content = benchContent(iw, ih)
	b.ReportAllocs()
	for b.Loop() {
		_ = Panel(th, o)
	}
}

func BenchmarkDiffLine(b *testing.B) {
	th := themes()["truecolor"]
	spans := []Span{
		{Text: "\ttok, err := ", Fg: th.Syntax.Variable},
		{Text: "s.cache.Get", Fg: th.Syntax.Function, Emph: true},
		{Text: "(ctx, s.userID, s.client.Token)", Fg: th.Syntax.Plain, Emph: true},
		{Text: " // cached per user", Fg: th.Syntax.Comment},
	}
	o := DiffLineOptions{Width: 84, Kind: DiffAdd, New: 44, Spans: spans}
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffLine(th, o)
	}
}

func BenchmarkWidth(b *testing.B) {
	s := Lines(benchContent(90, 4))[0] + "│ ⠹ auth-refactor · working ─────"
	b.ReportAllocs()
	for b.Loop() {
		_ = Width(s)
	}
}

func BenchmarkBackfill(b *testing.B) {
	th := themes()["truecolor"]
	s := benchContent(90, 47)
	b.ReportAllocs()
	for b.Loop() {
		_ = Backfill(s, th.Surface.Base)
	}
}
