package main

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// agentTranscript fakes the agent CLI's own terminal (Claude Code style). In
// the app this is the VT emulator's screen, backfilled onto the panel.
func (s scene) agentTranscript(w, h int) string {
	t := s.t
	dot := func(c kit.Tone) string { return paint(kit.ToneColor(t, c), false, "●") }
	tool := func(name, arg string) string {
		return " " + dot(kit.ToneSuccess) + " " + paint(t.Text.Strong, true, name) + paint(t.Text.Secondary, false, "("+arg+")")
	}
	out := func(text string) string {
		return "   " + paint(t.Text.Faint, false, "⎿  ") + paint(t.Text.Muted, false, text)
	}
	say := func(text string) []string {
		var ls []string
		for i, l := range kit.Lines(ansiWrap(text, w-4)) {
			lead := "   "
			if i == 0 {
				lead = " " + paint(t.Text.Primary, false, "●") + " "
			}
			ls = append(ls, lead+paint(t.Text.Primary, false, l))
		}
		return ls
	}
	lines := []string{
		"",
		tool("Read", "src/auth/session.go"),
		out("Read 182 lines"),
		"",
		tool("Update", "src/auth/session.go"),
		out("Updated with 24 additions and 9 removals"),
		"",
		tool("Bash", "go test ./internal/auth/..."),
		out("ok   internal/auth   0.412s"),
		"",
		tool("Update", "src/auth/cache.go"),
		out("Created with 41 lines"),
		"",
	}
	lines = append(lines, say("Token cache is in place and the auth tests pass. Before I update the callers in api/ and worker/: should the cache be per-user or global? I raised it as card 1.")...)
	lines = append(lines, "")
	frames := []string{"✻", "✽", "✶", "✳"}
	lines = append(lines, " "+paint(t.State.Blocked, false, frames[s.frame%len(frames)])+" "+
		paint(t.State.Blocked, false, "Updating callers…")+paint(t.Text.Faint, false, " (esc to interrupt)"))

	// Input box pinned to the bottom, as the CLI draws it.
	rule := paint(t.Text.Faint, false, strings.Repeat("─", max(w-2, 0)))
	bottom := []string{
		" " + rule,
		" " + paint(t.Text.Muted, false, "> ") + paint(t.Text.Faint, false, "Try \"update the callers in api/\""),
		" " + rule,
		"   " + paint(t.Text.Faint, false, "? for shortcuts"),
	}
	for len(lines)+len(bottom) < h {
		lines = append(lines, "")
	}
	if len(lines)+len(bottom) > h {
		lines = lines[len(lines)+len(bottom)-h:]
	}
	return strings.Join(append(lines, bottom...), "\n")
}
