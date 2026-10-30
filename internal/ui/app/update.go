package app

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/pane"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// ProcessStart is when the process started, as well as the app package knows:
// its initialisation time. `mpx bench startup` measures from it, and a caller
// that has an earlier timestamp may set it before Run.
var ProcessStart = time.Now()

// benchDoneMsg ends a first-frame benchmark run.
type benchDoneMsg struct{ at time.Time }

// Update handles one message. Every message takes the same path: dispatch,
// then recompute the layout and arm the animation tick if something animates.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	if m.quit {
		return m, cmd
	}
	m.recompute()
	return m, tea.Batch(cmd, m.announce(), m.tickCmd())
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	m.noteChange(msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		first := m.w == 0
		m.w, m.h = msg.Width, msg.Height
		m.recompute()
		m.sbEnsureVisible()
		if first && m.bench.on {
			return m.benchWait()
		}
		return nil
	case loadedMsg:
		return m.handleLoaded(msg)
	case eventsMsg:
		return m.handleEvents(msg)
	case tickMsg:
		return m.handleTick()
	case clockMsg:
		m.mainVer++ // views draw ages from the clock
		return clockCmd()
	case toastExpireMsg:
		m.removeToast(msg.id)
		return nil
	case errMsg:
		return m.addToast(toastSpec{tone: kit.ToneDanger, title: msg.what + " failed", body: msg.err.Error()})
	case pane.Msg:
		return msg.Dest().Update(msg)
	case view.SwitchModeMsg:
		return m.switchMode(msg)
	case view.ToastMsg:
		return m.addToast(toastSpec{tone: msg.Tone, title: msg.Title, body: msg.Body})
	case sessionCreatedMsg:
		return m.handleCreated(msg)
	case approvalResultMsg:
		return m.handleApprovalResult(msg)
	case specTaskMsg:
		return m.routeToOverlay(msg)
	case tea.FocusMsg:
		m.blurred = false
		return nil
	case tea.BlurMsg:
		m.blurred = true
		return nil
	case benchDoneMsg:
		fmt.Fprintf(os.Stderr, "first_frame=%s\n", msg.at.Sub(ProcessStart))
		m.quit = true
		return tea.Quit
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		return m.handlePaste(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	// Anything else is a view's own message (the result of one of its
	// commands): hand it to every built view, which ignore what is not theirs.
	return m.broadcast(msg)
}

// benchWait returns the command that finishes a first-frame benchmark after
// the first full frame was rendered.
func (m *Model) benchWait() tea.Cmd {
	drawn := m.bench.drawn
	return func() tea.Msg {
		<-drawn
		return benchDoneMsg{at: time.Now()}
	}
}

// noteChange bumps the shell-state version for every message except the
// animation tick and batches made only of pane frames, which change nothing
// the sidebar or main panel show.
func (m *Model) noteChange(msg tea.Msg) {
	switch msg := msg.(type) {
	case tickMsg:
		return
	case eventsMsg:
		for _, ev := range msg.events {
			if ev.Topic != protocol.TopicPaneFrame {
				m.ver++
				return
			}
		}
		if msg.closed {
			m.ver++
		}
		return
	}
	m.ver++
}
