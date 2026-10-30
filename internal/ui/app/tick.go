package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// tickMsg is the animation tick.
type tickMsg struct{}

// needsTick reports whether anything animates right now: a working session's
// spinner, a view that asks for ticks, or the blinking approvals dot. With
// reduced motion only views can ask. While an overlay is open the screen
// behind is dimmed and still, and when the terminal window lost focus nothing
// animates, so an idle app costs no CPU.
func (m *Model) needsTick() bool {
	if m.blurred || len(m.overlays) > 0 || m.lay.tooSmall || !m.loaded {
		return false
	}
	if v, ok := m.views[m.mode]; ok && v.Animating() {
		return true
	}
	if m.th.Motion.Reduce {
		return false
	}
	for _, s := range m.sessions {
		if s.State == core.StateWorking || s.State == core.StateStarting {
			return true
		}
	}
	return m.th.Animated() && len(m.approvals) > 0
}

// tickCmd arms the tick if something animates and no tick is pending.
func (m *Model) tickCmd() tea.Cmd {
	if m.tickArmed || !m.needsTick() {
		return nil
	}
	m.tickArmed = true
	return tea.Tick(m.th.Motion.SpinnerInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// handleTick advances the frame and tells animating views.
func (m *Model) handleTick() tea.Cmd {
	m.tickArmed = false
	m.frame++
	var cmds []tea.Cmd
	if v, ok := m.views[m.mode]; ok && v.Animating() {
		cmds = append(cmds, m.sendView(m.mode, view.TickMsg{Frame: m.frame}))
	}
	return tea.Batch(cmds...)
}

// clockInterval is how often the screen refreshes ages ("updated 32s ago",
// "implementer · 14m") while nothing else happens. It is slow on purpose: an
// idle app wakes twice a minute.
const clockInterval = 30 * time.Second

// clockMsg refreshes time-dependent text.
type clockMsg struct{}

func clockCmd() tea.Cmd {
	return tea.Tick(clockInterval, func(time.Time) tea.Msg { return clockMsg{} })
}
