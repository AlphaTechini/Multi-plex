package diffview

import "github.com/alphatechini/multi-plex/internal/ui/kit"

// renderState draws the empty states: no session, loading, error and no
// changes. ok is false when the view has a file to show.
func (m *Model) renderState(w, h int) (string, bool) {
	o := kit.EmptyStateOptions{Width: w, Height: h}
	switch {
	case m.session == nil:
		o.Title = "No session selected"
		o.Body = "Select a session in the sidebar to review what its agent changed."
	case !m.loaded && m.loadErr != "":
		o.Title = "Could not read the diff"
		o.Body = m.loadErr
	case !m.loaded:
		o.Title = "Reading the diff"
		o.Body = "Collecting the changes in " + m.sessionName() + "."
	case len(m.files) == 0 || m.fv == nil:
		o.Eyebrow = m.sessionName()
		o.Title = "No changes yet"
		o.Body = "The diff appears here as the agent edits files."
	default:
		return "", false
	}
	return kit.EmptyState(m.t, o), true
}
