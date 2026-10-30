package codeview

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// renderEmpty is the view with no file open: the follow row, then a calm
// explanation.
func (m *model) renderEmpty(w, h int) string {
	th := m.th
	o := kit.EmptyStateOptions{Width: w, Height: h}
	if m.follow {
		o.Height = h - 1
		o.Title = "Waiting for the next edit"
		o.Body = "Follow mode is on. The file the agent edits opens here, with the new lines highlighted."
		o.Hints = []kit.Hint{{Keys: "f", Label: "stop following"}, {Keys: "p", Label: "go to file"}}
	} else {
		o.Title = "No file open"
		o.Body = "Open one of this session's changed files, or turn on follow mode to watch the agent write code."
		o.Hints = []kit.Hint{{Keys: "p", Label: "go to file"}, {Keys: "f", Label: "follow"}}
	}
	if m.follow {
		return m.statusRow(w) + "\n" + kit.EmptyState(th, o)
	}
	return kit.EmptyState(th, o)
}

// renderNote is the view for a path that has no text to show: a global file,
// a binary or oversized file, an error, or a file still opening.
func (m *model) renderNote(w, h int) string {
	th := m.th
	o := kit.EmptyStateOptions{Width: w, Height: h - headerRows}
	switch {
	case m.note == "global":
		o.Title = "Global instruction file"
		o.Body = clean(m.path, true) + " lives outside the workspace, so it opens in your editor."
		o.Hints = []kit.Hint{{Keys: "e", Label: "open in editor"}}
	case m.note == "binary":
		o.Title = "Binary file"
		o.Body = "Multiplex does not preview binary files."
	case m.note == "large":
		o.Title = "File too large to preview"
		o.Body = "Files over 5 MB open in your editor instead."
		o.Hints = []kit.Hint{{Keys: "e", Label: "open in editor"}}
	case strings.HasPrefix(m.note, "error:"):
		o.Title = "Could not open the file"
		o.Body = strings.TrimPrefix(m.note, "error:")
	default:
		o.Title = "Opening…"
	}
	return m.banner(w) + "\n" + m.statusRow(w) + "\n" + kit.EmptyState(th, o)
}
