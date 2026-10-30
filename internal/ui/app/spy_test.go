package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

type backendEvent = backend.Event

func themeForTest() *theme.Theme { return theme.DarkPlus() }

// spyView records every message it receives.
type spyView struct {
	env       view.Env
	mode      view.Mode
	log       []tea.Msg
	capturing bool
}

func (s *spyView) Update(msg tea.Msg) (view.View, tea.Cmd) {
	s.log = append(s.log, msg)
	return s, nil
}
func (s *spyView) Render(w, h int) string {
	return kit.FitBlock("spy "+modeName(s.mode), w, h)
}
func (s *spyView) Tab() (string, int) { return modeName(s.mode), 2 }
func (s *spyView) Hints() []kit.Hint  { return []kit.Hint{{Keys: "x", Label: "spy hint"}} }
func (s *spyView) Animating() bool    { return false }
func (s *spyView) Capturing() bool    { return s.capturing }

// saw reports whether a key press with this name was received.
func (s *spyView) saw(key string) bool {
	for _, m := range s.log {
		if k, ok := m.(tea.KeyPressMsg); ok && k.String() == key {
			return true
		}
	}
	return false
}

// installSpy replaces the constructor of mode for the test.
func installSpy(t *testing.T, mode view.Mode) *spyView {
	t.Helper()
	spy := &spyView{mode: mode}
	old := modeConstructors[mode]
	modeConstructors[mode] = func(env view.Env) view.View { spy.env = env; return spy }
	t.Cleanup(func() { modeConstructors[mode] = old })
	return spy
}
