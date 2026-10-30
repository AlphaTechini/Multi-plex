package app

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

func benchRender(b *testing.B, w, h int) {
	hn := newHarness(b, w, h)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		hn.m.frame = i // a new spinner frame every render
		_ = hn.m.render()
	}
}

// BenchmarkRender180x48 renders the full screen: the target is under 2 ms.
func BenchmarkRender180x48(b *testing.B) { benchRender(b, 180, 48) }

// BenchmarkRender110x32 renders the compact layout.
func BenchmarkRender110x32(b *testing.B) { benchRender(b, 110, 32) }

// BenchmarkRenderStreaming is the hot path while an agent prints: one pane
// frame per render, every other region cached.
func BenchmarkRenderStreaming(b *testing.B) {
	h := newHarness(b, 180, 48)
	p := h.m.agentPaneOf(h.m.session())
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		p.Apply(protocol.PaneFrame{PaneID: p.ID(), Cols: 58, Rows: 46, Changed: []protocol.Row{{Y: i % 40, Text: kit.Fit("\x1b[38;5;244mline "+itoa(i)+"\x1b[0m", 58)}}})
		_ = h.m.render()
	}
}

// BenchmarkSwitch is a key press that changes the session or the mode, plus
// the render that follows: the budget is one frame (16 ms).
func BenchmarkSwitchSession(b *testing.B) {
	h := newHarness(b, 180, 48)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = h.m.Update(keyMsg("ctrl+\\"))
		_, _ = h.m.Update(keyMsg("]"))
		_ = h.m.render()
	}
}

func BenchmarkSwitchMode(b *testing.B) {
	h := newHarness(b, 180, 48)
	keys := []string{"2", "1", "3", "1", "4", "1"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		_, _ = h.m.Update(keyMsg("ctrl+\\"))
		_, _ = h.m.Update(keyMsg(keys[i%len(keys)]))
		_ = h.m.render()
	}
}
