package diffview

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// animState drives the file-switch banner and fade and the new-hunk
// highlight. Progress is computed from the clock when a tick arrives, so the
// shell's tick rate does not change the timing; Render reads the results.
type animState struct {
	switching  bool
	start      time.Time
	fadeStep   int     // 0..FadeSteps; FadeSteps is settled
	settle     float64 // banner settle 0..1
	freshOn    bool
	freshStart time.Time
	fresh      float64 // new-hunk highlight 1..0
}

// startSwitch begins the banner tint and the content fade. Frame 0 of the
// fade (everything dimmed) is what the next Render shows, so the switch is
// drawn in the same update.
func (m *Model) startSwitch() {
	if !m.t.Animated() {
		m.anim.switching, m.anim.fadeStep, m.anim.settle = false, m.t.Motion.FadeSteps, 1
		return
	}
	m.anim.switching = true
	m.anim.start = m.now()
	m.anim.fadeStep = 0
	m.anim.settle = 0
}

// finishFade completes the fade at once (a key was pressed mid-fade).
func (m *Model) finishFade() {
	if m.anim.switching {
		m.anim.fadeStep = m.t.Motion.FadeSteps
	}
}

// startFresh begins the new-hunk highlight decay.
func (m *Model) startFresh() {
	m.anim.freshOn = true
	m.anim.freshStart = m.now()
	m.anim.fresh = 1
}

// clearFresh drops the new-hunk highlight (scrolled or switched with reduced
// motion, or the decay ended).
func (m *Model) clearFresh() {
	m.anim.freshOn, m.anim.fresh = false, 0
	clear(m.fresh)
	if m.fv != nil {
		m.fv.gen++
	}
}

// advanceAnim moves the animations to the clock time now.
func (m *Model) advanceAnim(now time.Time) {
	mo := m.t.Motion
	if m.anim.switching {
		el := now.Sub(m.anim.start)
		stepDur := mo.FadeDuration / time.Duration(max(mo.FadeSteps, 1))
		step := mo.FadeSteps
		if stepDur > 0 {
			step = min(int(el/stepDur), mo.FadeSteps)
		}
		m.anim.fadeStep = max(m.anim.fadeStep, step)
		m.anim.settle = 1
		if mo.BannerHold > 0 {
			m.anim.settle = min(float64(el)/float64(mo.BannerHold), 1)
		}
		if m.anim.fadeStep >= mo.FadeSteps && m.anim.settle >= 1 {
			m.anim.switching = false
		}
	}
	if m.anim.freshOn && m.t.Animated() {
		el := now.Sub(m.anim.freshStart)
		steps := max(mo.HunkDecaySteps, 1)
		stepDur := mo.HunkDecay / time.Duration(steps)
		step := steps
		if stepDur > 0 {
			step = min(int(el/stepDur), steps)
		}
		if step >= steps {
			m.clearFresh()
			return
		}
		next := 1 - theme.EaseOutCubic(float64(step)/float64(steps))
		if next != m.anim.fresh {
			m.anim.fresh = next
			if m.fv != nil {
				m.fv.gen++
			}
		}
	}
}

// fadeAmount is the dim amount of the content below the banner (1 = fully
// faded to the background).
func (m *Model) fadeAmount() float64 {
	if !m.anim.switching || !m.t.Animated() {
		return 0
	}
	steps := max(m.t.Motion.FadeSteps, 1)
	return 1 - theme.EaseOutCubic(float64(m.anim.fadeStep)/float64(steps))
}

// bannerSettle is the banner animation progress passed to kit.Banner.
func (m *Model) bannerSettle() float64 {
	if !m.anim.switching || !m.t.Animated() {
		return 1
	}
	return m.anim.settle
}
