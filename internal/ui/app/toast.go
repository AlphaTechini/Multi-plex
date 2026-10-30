package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

const (
	toastWidth = 46
	maxToasts  = 3
)

// toastSpec describes a toast to show.
type toastSpec struct {
	tone  kit.Tone
	title string
	body  string
	hints []kit.Hint
	// sticky toasts (warnings and errors) stay until dismissed.
	sticky bool
}

// toast is a shown toast.
type toast struct {
	id int
	toastSpec
}

// toastExpireMsg removes a timed toast.
type toastExpireMsg struct{ id int }

// addToast shows a toast; info and success toasts leave after the theme's
// toast duration, the others stay. At most three are shown, the oldest is
// pushed out.
func (m *Model) addToast(s toastSpec) tea.Cmd {
	if s.tone == kit.ToneWarning || s.tone == kit.ToneDanger {
		s.sticky = true
	}
	m.toastSeq++
	t := toast{id: m.toastSeq, toastSpec: s}
	m.toasts = append(m.toasts, t)
	if len(m.toasts) > maxToasts {
		m.toasts = m.toasts[len(m.toasts)-maxToasts:]
	}
	if s.sticky {
		return nil
	}
	id := t.id
	return tea.Tick(m.th.Motion.ToastDuration, func(time.Time) tea.Msg { return toastExpireMsg{id: id} })
}

func (m *Model) removeToast(id int) {
	for i, t := range m.toasts {
		if t.id == id {
			m.toasts = append(m.toasts[:i], m.toasts[i+1:]...)
			return
		}
	}
}

// toastRects returns each toast's rendered block and its screen position,
// bottom-right of the main view, newest at the bottom.
func (m *Model) toastRects() (blocks []string, xs, ys []int) {
	l := m.lay
	if len(m.toasts) == 0 || l.tooSmall || l.mainW == 0 {
		return nil, nil, nil
	}
	w := min(toastWidth, l.mainW-4)
	x := l.x(regionMain) + l.mainW - w - 2
	if l.drawer {
		// The agent drawer covers the right of the main view: draw toasts
		// inside it, so they never straddle its frame.
		w = min(toastWidth, l.drawerW-4)
		x = l.w - l.drawerW + 2
	}
	if w < 20 {
		return nil, nil, nil
	}
	y := l.bodyH - 1
	for i := len(m.toasts) - 1; i >= 0; i-- {
		ts := m.toasts[i]
		b := kit.Toast(m.th, kit.ToastOptions{Width: w, Tone: ts.tone, Title: ts.title, Body: ts.body, Hints: ts.hints})
		y -= len(kit.Lines(b))
		if y < 1 {
			break
		}
		blocks, xs, ys = append(blocks, b), append(xs, x), append(ys, y)
	}
	return blocks, xs, ys
}

// compositeToasts draws the toasts over the screen.
func (m *Model) compositeToasts(screen string) string {
	blocks, xs, ys := m.toastRects()
	for i := range blocks {
		screen = kit.Composite(screen, blocks[i], xs[i], ys[i])
	}
	return screen
}

// toastAt returns the ID of the toast under (x, y), or 0.
func (m *Model) toastAt(x, y int) int {
	blocks, xs, ys := m.toastRects()
	n := len(m.toasts)
	for i := range blocks {
		h := len(strings.Split(blocks[i], "\n"))
		if x >= xs[i] && x < xs[i]+kit.Width(strings.SplitN(blocks[i], "\n", 2)[0]) && y >= ys[i] && y < ys[i]+h {
			return m.toasts[n-1-i].id
		}
	}
	return 0
}
