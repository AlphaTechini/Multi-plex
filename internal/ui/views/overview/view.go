// Package overview is the Overview main-view mode (docs/08, docs/16): the
// selected session's header, its latest structured report, numbered insight
// cards, conflict warnings, spec progress and checkpoint/approval counts.
package overview

import (
	"strconv"
	"time"

	"charm.land/bubbles/v2/textinput"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// zone is the part of the view the keyboard selection is in.
type zone uint8

const (
	zoneCards zone = iota
	zoneFiles
)

// model is the Overview view. Update and Render run on the Bubble Tea
// goroutine; Render also records layout state (hit rectangles, card window
// and file count) that the mouse and selection handlers read, which is why it
// is a pointer type.
type model struct {
	th *theme.Theme
	be backend.Backend
	ws core.Workspace

	// now is the clock; tests replace it. clock is the time of the last
	// Update, which Render uses so it stays pure.
	now   func() time.Time
	clock time.Time

	frame   int
	focused bool

	sess       *core.Session
	detail     protocol.SessionDetail
	hasDetail  bool
	loading    bool
	dirty      bool
	dirtySpecs bool
	loadErr    string
	loadSeq    int
	names      map[core.SessionID]string
	spec       *protocol.SpecInfo

	checkedAt time.Time

	cards     []core.Note // visible cards, newest first
	dismissed map[string]bool
	seen      map[string]bool
	sel       int
	zone      zone
	fileSel   int

	in *input // non-nil while the ask/steer box is open

	// Render state.
	top       int
	nFiles    int // selectable file links, "+N more" included
	nVisFiles int // file links shown
	fileList  []string
	rows      []string
	hits      []hit
}

// New returns the Overview view.
func New(env view.Env) view.View {
	m := &model{
		th:        env.Theme,
		be:        env.Backend,
		ws:        env.Workspace,
		now:       time.Now,
		dismissed: map[string]bool{},
		seen:      map[string]bool{},
		names:     map[core.SessionID]string{},
	}
	m.clock = m.now()
	return m
}

// Tab returns "Overview" with the number of undismissed cards as its badge.
func (m *model) Tab() (string, int) { return "Overview", len(m.cards) }

// Animating reports whether the working spinner needs ticks.
func (m *model) Animating() bool {
	return m.hasDetail && m.state() == kit.StateWorking && !m.th.Motion.Reduce
}

// Capturing reports whether the ask/steer text box wants raw keys.
func (m *model) Capturing() bool { return m.in != nil }

// Hints returns the key hints for the status bar.
func (m *model) Hints() []kit.Hint {
	if m.in != nil {
		return []kit.Hint{{Keys: "enter", Label: "send"}, {Keys: "esc", Label: "cancel"}}
	}
	if m.zone == zoneFiles {
		return []kit.Hint{{Keys: "left right", Label: "file"}, {Keys: "enter", Label: "open diff"}, {Keys: "esc", Label: "cards"}}
	}
	h := []kit.Hint{{Keys: "up down", Label: "cards"}, {Keys: "enter", Label: "open ref"}}
	if m.nFiles > 0 {
		h = append(h, kit.Hint{Keys: "f", Label: "files"})
	}
	return h
}

// state maps the session state onto the kit's.
func (m *model) state() kit.State {
	switch m.detail.Session.State {
	case core.StateStarting:
		return kit.StateStarting
	case core.StateWorking:
		return kit.StateWorking
	case core.StateBlocked:
		return kit.StateBlocked
	case core.StateDone:
		return kit.StateDone
	case core.StateError:
		return kit.StateError
	default:
		return kit.StateIdle
	}
}

// newInput builds the ask/steer text box state.
func newInput(th *theme.Theme, action core.NoteAction, noteID string, number int) *input {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 2000
	st := textinput.DefaultStyles(true)
	text := st.Focused.Text.Foreground(th.Text.Primary)
	ph := st.Focused.Placeholder.Foreground(th.Text.Faint)
	st.Focused.Text, st.Blurred.Text = text, text
	st.Focused.Placeholder, st.Blurred.Placeholder = ph, ph
	st.Cursor.Color = th.Accent.Base
	st.Cursor.Blink = false
	ti.SetStyles(st)
	ti.Placeholder = placeholder(action)
	ti.Focus()
	return &input{ti: ti, action: action, noteID: noteID, number: number}
}

// placeholder is the faint hint inside an empty box.
func placeholder(a core.NoteAction) string {
	switch a {
	case core.NoteAsk:
		return "Ask a question about this card"
	case core.NoteInterruptSteer:
		return "Message to send right after interrupting"
	default:
		return "Message for the agent's next turn"
	}
}

// cardLabel names the box: "Ask about card 2".
func cardLabel(a core.NoteAction, n int) string {
	num := strconv.Itoa(n)
	switch a {
	case core.NoteAsk:
		return "Ask about card " + num
	case core.NoteInterruptSteer:
		return "Interrupt and steer, card " + num
	default:
		return "Steer, card " + num
	}
}
