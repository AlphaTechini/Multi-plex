package main

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

var cards = []kit.CardOptions{
	{
		Number: 1, Severity: kit.SeverityDecision, Age: "2m", Unread: true,
		Title: "Cache scope: per-user or global?",
		Body:  "A per-user cache costs about 4 MB at 10k users. A global cache is smaller but would share tokens across tenants if a key is ever wrong. I am holding the caller updates until you decide.",
		Refs:  []string{"src/auth/cache.go:L12-L30"},
	},
	{
		Number: 2, Severity: kit.SeverityWarning, Age: "6m", Unread: true,
		Title: "Changing Session.Refresh signature",
		Body:  "Refresh now takes a cache argument. Callers in api/ and worker/ need updating; api-pagination also edits src/api/client.go.",
		Refs:  []string{"src/auth/session.go:L38-L52"},
	},
	{
		Number: 3, Severity: kit.SeverityInfo, Age: "11m",
		Title: "Assumed tokens expire after 5 minutes",
		Body:  "Matched the TTL to the identity provider's default token lifetime. Override with AUTH_CACHE_TTL.",
		Refs:  []string{"src/auth/cache.go:L8"},
	},
}

var cardActions = []kit.Hint{
	{Keys: "a", Label: "ask"}, {Keys: "s", Label: "steer"}, {Keys: "i", Label: "interrupt + steer"}, {Keys: "d", Label: "dismiss"},
}

func (s scene) overview() string {
	return s.screen(screenOpts{mode: 0, focus: focusMain}, s.overviewBody, "updated 32s ago")
}

// overviewBody renders the Overview mode at the main panel's inner size.
func (s scene) overviewBody(w, h int) string {
	t := s.t
	const pad = 2
	cw := w - 2*pad
	var lines []string
	add := func(block string) {
		for _, l := range strings.Split(block, "\n") {
			lines = append(lines, strings.Repeat(" ", pad)+l)
		}
	}
	gap := func() { lines = append(lines, "") }

	gap()
	add(kit.Fit(paint(t.Text.Strong, true, "Cache auth tokens per user and update the callers"), cw))
	add(kit.Fit(kit.StateBadge(t, kit.StateWorking, s.frame)+s.dot()+s.ink(kit.ToneMuted, "implementer")+s.dot()+
		s.ink(kit.ToneMuted, t.Glyphs.Branch+" mpx/auth-refactor")+s.dot()+s.ink(kit.ToneMuted, "turn 6 · 14m"), cw))
	gap()

	compact := h < 40
	add(kit.Section(t, kit.SectionOptions{Width: cw + 2, Label: "Summary", Meta: "report 6 of 6"})[1:])
	kv := func(k string, vals ...string) {
		for i, v := range vals {
			key := ""
			if i == 0 {
				key = k
			}
			add(kit.Fit(kit.PadRight(s.ink(kit.ToneMuted, key), 12)+v, cw))
		}
	}
	check := paint(t.State.Done, false, t.Glyphs.Check) + " "
	if compact {
		kv("Done", check+s.ink(kit.ToneDefault, "Added token cache to Session.Refresh; moved expiry check into tokenValid"))
	} else {
		kv("Done", check+s.ink(kit.ToneDefault, "Added token cache to Session.Refresh"),
			check+s.ink(kit.ToneDefault, "Moved the expiry check into tokenValid()"))
		kv("Decided", s.ink(kit.ToneDefault, "Cache TTL is 5 minutes, matching the token lifetime"))
	}
	kv("Open", paint(t.Severity.Decision, false, "?")+" "+s.ink(kit.ToneDefault, "Should the cache be per-user or global?"))
	kv("Next", s.ink(kit.ToneDefault, "Update callers in api/ and worker/"))
	kv("Files", s.ink(kit.ToneAccent, "src/auth/session.go")+"  "+s.ink(kit.ToneAccent, "src/auth/cache.go")+"  "+s.ink(kit.ToneMuted, "+5 more"))
	gap()

	add(kit.Section(t, kit.SectionOptions{Width: cw + 2, Label: "Insights", Meta: "2 new", MetaTone: kit.ToneAccent})[1:])
	tail := []string{}
	{
		var tl []string
		tl = append(tl, "")
		tl = append(tl, kit.Section(t, kit.SectionOptions{Width: cw + 2, Label: "Conflicts"})[1:])
		tl = append(tl, kit.Fit(paint(t.Severity.Warning, false, t.Glyphs.Blocked)+" "+s.ink(kit.ToneDefault, "Will conflict with ")+
			paint(t.Text.Strong, true, "api-pagination")+s.ink(kit.ToneDefault, " in ")+s.ink(kit.ToneAccent, "src/api/client.go")+
			s.ink(kit.ToneMuted, "   merge-tree, checked 1m ago"), cw))
		tl = append(tl, "")
		tl = append(tl, kit.Section(t, kit.SectionOptions{Width: cw + 2, Label: "Spec"})[1:])
		tl = append(tl, kit.Fit(kit.PadRight(s.ink(kit.ToneDefault, "token-refresh"), 16)+
			kit.Progress(t, kit.ProgressOptions{Width: 28, Done: 3, Total: 7, Label: true})+s.ink(kit.ToneMuted, "  tasks   ")+
			s.ink(kit.ToneMuted, "5/9 criteria verified"), cw))
		for _, l := range tl {
			tail = append(tail, strings.Repeat(" ", pad)+l)
		}
	}
	room := h - len(lines) - len(tail)
	shown := 0
	for i, c := range cards {
		c.Width = cw
		c.Actions = cardActions
		c.Selected = i == 0
		need := kit.CardHeight(c)
		if i > 0 {
			need++
		}
		if room-need < 1 && i < len(cards)-1 || room-need < 0 {
			break
		}
		if i > 0 {
			gap()
		}
		add(kit.Card(t, c))
		room -= need
		shown++
	}
	if more := len(cards) - shown; more > 0 {
		add(kit.Fit(s.ink(kit.ToneMuted, "   "+t.Glyphs.ArrowDown+" "+itoa(more)+" more card below"), cw))
	}
	lines = append(lines, tail...)
	return strings.Join(lines, "\n")
}

func (s scene) dot() string { return s.ink(kit.ToneFaint, "  "+s.t.Glyphs.Dot+"  ") }
