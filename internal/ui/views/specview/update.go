package specview

import (
	"path"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Update handles messages. It never does I/O; backend calls are returned as
// commands. The first message of any kind starts loading the spec list, so a
// view the shell creates lazily fills itself.
func (m *model) Update(msg tea.Msg) (view.View, tea.Cmd) {
	var cmds []tea.Cmd
	if m.list == notLoaded {
		m.list = loading
		cmds = append(cmds, specsCmd(m.be, m.ws.ID))
	}
	switch msg := msg.(type) {
	case view.FocusMsg:
		m.focused = msg.Focused
		if msg.Focused && m.list == loaded { // pick up spec changes made while away
			m.list = loading
			cmds = append(cmds, specsCmd(m.be, m.ws.ID), m.reloadDoc())
		}
	case view.SwitchModeMsg:
		if msg.Mode == view.ModeSpecs {
			cmds = append(cmds, m.onSwitch(msg.Path))
		}
	case specsMsg:
		cmds = append(cmds, m.onSpecs(msg))
	case docMsg:
		m.onDoc(msg)
	case tea.KeyPressMsg:
		cmds = append(cmds, m.onKey(msg))
	case tea.MouseClickMsg:
		cmds = append(cmds, m.onClick(msg))
	case tea.MouseWheelMsg:
		cmds = append(cmds, m.onWheel(msg))
	}
	return m, tea.Batch(cmds...)
}

// onSpecs applies the spec list, keeping the selection on the same spec.
func (m *model) onSpecs(msg specsMsg) tea.Cmd {
	if msg.err != nil {
		m.list, m.listErr = failed, clean(msg.err.Error(), true)
		return nil
	}
	keep := ""
	if s, ok := m.current(); ok {
		keep = s.Slug
	}
	if m.want != "" {
		keep, m.want = m.want, ""
	}
	m.list, m.listErr = loaded, ""
	m.specs = msg.specs
	m.sel = 0
	for i, s := range m.specs {
		if s.Slug == keep {
			m.sel = i
			break
		}
	}
	return m.ensureDoc()
}

// onDoc stores a spec's documents.
func (m *model) onDoc(msg docMsg) {
	d := m.docs[msg.slug]
	if d == nil {
		d = &doc{}
		m.docs[msg.slug] = d
	}
	if msg.err != nil {
		d.state, d.err = failed, clean(msg.err.Error(), true)
		return
	}
	d.state, d.err, d.spec = loaded, "", msg.doc
	d.lines = [3][]mdLine{}
	d.width = 0
}

// ensureDoc loads the selected spec's documents when they are not there yet.
func (m *model) ensureDoc() tea.Cmd {
	s, ok := m.current()
	if !ok {
		return nil
	}
	d := m.docs[s.Slug]
	if d != nil && (d.state == loaded || d.state == loading) {
		return nil
	}
	if d == nil {
		d = &doc{}
		m.docs[s.Slug] = d
	}
	d.state = loading
	return docCmd(m.be, m.ws.ID, s.Slug)
}

// reloadDoc refetches the selected spec's documents (the content may have
// changed on disk) without clearing what is shown meanwhile.
func (m *model) reloadDoc() tea.Cmd {
	s, ok := m.current()
	if !ok {
		return nil
	}
	if d := m.docs[s.Slug]; d != nil && d.state == loaded {
		return docCmd(m.be, m.ws.ID, s.Slug)
	}
	return m.ensureDoc()
}

// onSwitch selects the spec (and document) a shell link names: a slug, a
// "specs/<slug>" path, or ".multiplex/specs/<slug>/tasks.md".
func (m *model) onSwitch(p string) tea.Cmd {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	if p == "" {
		return nil
	}
	switch path.Base(p) {
	case "requirements.md":
		m.kind, p = docRequirements, path.Dir(p)
	case "design.md":
		m.kind, p = docDesign, path.Dir(p)
	case "tasks.md":
		m.kind, p = docTasks, path.Dir(p)
	}
	slug := path.Base(strings.TrimSuffix(p, "/"))
	m.detail = true
	for i, s := range m.specs {
		if s.Slug == slug {
			m.sel = i
			return m.ensureDoc()
		}
	}
	m.want = slug // the list has not arrived yet
	return nil
}
