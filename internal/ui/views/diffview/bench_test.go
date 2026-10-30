package diffview

import (
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/highlight/chromahl"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// synthBig builds a session whose first file has a diff of about n lines
// (hunks of 20 lines) followed by a small second file.
func synthBig(n int) *fakeBackend {
	be := newFake()
	var hs []string
	var content strings.Builder
	for i := 0; i < n/20; i++ {
		start := 1 + i*40
		var ls []string
		for k := 0; k < 5; k++ {
			ls = append(ls, " \tctx := trace.With(ctx, \"handler"+itoa(k)+"\")")
		}
		for k := 0; k < 4; k++ {
			ls = append(ls, "-\tresult, err := store.Get(ctx, req.ID, opts"+itoa(k)+"...)")
		}
		for k := 0; k < 6; k++ {
			ls = append(ls, "+\tresult, err := store.Fetch(ctx, req.ID, opts"+itoa(k)+"..., WithTimeout(time.Second))")
		}
		for k := 0; k < 5; k++ {
			ls = append(ls, " \tif err != nil { return fmt.Errorf(\"handler %d: %w\", "+itoa(k)+", err) }")
		}
		hs = append(hs, hunkText(start, start, "func handler"+itoa(i)+"(ctx context.Context) error", ls...))
	}
	be.raw = fileDiff("pkg/big.go", hs...) + fileDiff("pkg/small.go", hunkText(1, 1, "", " a", "-b", "+c", " d"))
	for i := 0; i < n/20*40+50; i++ {
		content.WriteString("\tx := " + itoa(i) + " // line\n")
	}
	be.files["pkg/big.go"] = content.String()
	be.files["pkg/small.go"] = "a\nc\nd\n"
	return be
}

// bigHarness opens the 10k-line file with real chroma highlighting and waits
// for the first highlight pass, like the view does in use.
func bigHarness(tb testing.TB, th *theme.Theme) *harness {
	be := synthBig(10000)
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	m := newModel(view.Env{Theme: th, Backend: be}, chromahl.New(), clock.now)
	h := &harness{m: m, be: be, clock: clock, skipTick: true}
	switch x := tb.(type) {
	case *testing.T:
		h.t = x
	case *testing.B:
		h.t = &testing.T{}
	}
	h.send(view.FocusMsg{Focused: true})
	h.send(view.SessionMsg{Session: testSession})
	m.Render(112, 45)
	return h
}

func BenchmarkRender10k(b *testing.B) {
	h := bigHarness(b, theme.DarkPlus())
	for i := 0; i < 40; i++ {
		h.key("ctrl+d")
	}
	h.render(112, 45)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.m.Render(112, 45)
	}
}

// BenchmarkRender10kCold draws every visible row from scratch each frame.
func BenchmarkRender10kCold(b *testing.B) {
	h := bigHarness(b, theme.DarkPlus())
	for i := 0; i < 40; i++ {
		h.key("ctrl+d")
	}
	h.render(112, 45)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.m.fv.gen++
		_ = h.m.Render(112, 45)
	}
}

// BenchmarkScroll10k scrolls one line and draws, the usual frame while reading.
func BenchmarkScroll10k(b *testing.B) {
	h := bigHarness(b, theme.DarkPlus())
	h.render(112, 45)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.m.scrollBy(1)
		if h.m.fv.top >= h.m.maxTop() {
			h.m.fv.top = 0
		}
		_ = h.m.Render(112, 45)
	}
}

// BenchmarkFileSwitch10k switches between a 10k-line file and a small one
// (key handling and the first frame, without the fade).
func BenchmarkFileSwitch10k(b *testing.B) {
	h := bigHarness(b, reduced())
	h.render(112, 45)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.m.Update(keyMsg("]"))
		_ = h.m.Render(112, 45)
		_, _ = h.m.Update(keyMsg("["))
		_ = h.m.Render(112, 45)
	}
}

// BenchmarkFade draws one frame of the fade (dimmed body) for comparison.
func BenchmarkFade10k(b *testing.B) {
	h := bigHarness(b, theme.DarkPlus())
	h.render(112, 45)
	h.m.anim.switching = true
	h.m.anim.fadeStep = 3
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.m.Render(112, 45)
	}
}

// TestRenderBudget is a coarse guard against a render that scans the file:
// the budget in docs/03 is 2 ms per frame; the bound here is wide so loaded CI
// machines do not flake, the benchmarks give the real numbers.
func TestRenderBudget(t *testing.T) {
	h := bigHarness(t, theme.DarkPlus())
	for i := 0; i < 12; i++ {
		h.key("ctrl+d")
	}
	h.render(112, 45)
	start := time.Now()
	const n = 50
	for i := 0; i < n; i++ {
		h.m.scrollBy(1)
		h.m.Render(112, 45)
	}
	if per := time.Since(start) / n; per > 25*time.Millisecond {
		t.Fatalf("render took %v per frame on a 10k-line diff", per)
	}
	// Switching files stays inside a frame or two as well.
	start = time.Now()
	for i := 0; i < 10; i++ {
		h.key("]")
		h.m.Render(112, 45)
		h.key("[")
		h.m.Render(112, 45)
	}
	if per := time.Since(start) / 20; per > 100*time.Millisecond {
		t.Fatalf("file switch took %v", per)
	}
}
