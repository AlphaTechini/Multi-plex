package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// placeholderView stands in for a mode whose real view is not wired yet. It
// renders the mode name and the selected session, and takes no keys.
type placeholderView struct {
	env     view.Env
	mode    view.Mode
	session string
}

// placeholder returns the constructor of a placeholder for mode.
func placeholder(mode view.Mode) func(view.Env) view.View {
	return func(env view.Env) view.View { return &placeholderView{env: env, mode: mode} }
}

func (p *placeholderView) Update(msg tea.Msg) (view.View, tea.Cmd) {
	if m, ok := msg.(view.SessionMsg); ok {
		p.session = ""
		if m.Session != nil {
			p.session = m.Session.Slug
		}
	}
	return p, nil
}

func (p *placeholderView) Render(w, h int) string {
	body := "This mode is not built yet."
	if p.session != "" {
		body += " Session: " + p.session + "."
	}
	return kit.EmptyState(p.env.Theme, kit.EmptyStateOptions{Width: w, Height: h, Eyebrow: "coming soon", Title: modeName(p.mode), Body: body})
}

func (p *placeholderView) Tab() (string, int) { return modeName(p.mode), 0 }
func (p *placeholderView) Hints() []kit.Hint  { return nil }
func (p *placeholderView) Animating() bool    { return false }
func (p *placeholderView) Capturing() bool    { return false }
