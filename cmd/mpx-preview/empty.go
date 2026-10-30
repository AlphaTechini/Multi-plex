package main

import "github.com/alphatechini/multi-plex/internal/ui/kit"

// empty is the first-run screen: no sessions yet.
func (s scene) empty() string {
	return s.screen(screenOpts{mode: 0, focus: focusMain, empty: true}, func(w, h int) string {
		return kit.EmptyState(s.t, kit.EmptyStateOptions{
			Width: w, Height: h,
			Eyebrow: "m u l t i p l e x",
			Title:   "No sessions yet",
			Body:    "A session is one agent working on one task in its own worktree and branch. Start one and its summary, diff and terminal appear here.",
			Hints: []kit.Hint{
				{Keys: "n", Label: "new session"},
				{Keys: "ctrl+\\ p", Label: "command palette"},
				{Keys: "?", Label: "all keys"},
			},
		})
	}, "")
}
