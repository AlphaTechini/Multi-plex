package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// helpOverlay lists every key.
type helpOverlay struct {
	scroll int
}

func newHelp(m *Model) *helpOverlay { return &helpOverlay{} }

// helpRow is one key and what it does.
type helpRow struct{ keys, label string }

// helpSections builds the help from the same action table the prefix and the
// palette use, plus the keys of the sidebar and the overlays.
func (m *Model) helpSections() (titles []string, rows [][]helpRow) {
	prefix := m.prefix
	group := func(name string) []helpRow {
		var out []helpRow
		for _, a := range staticActions {
			if a.group == name && len(a.keys) > 0 {
				keys := prefix + " " + a.keys[0]
				if len(a.keys) > 1 {
					keys += " / " + a.keys[1]
				}
				out = append(out, helpRow{keys, a.title})
			}
		}
		return out
	}
	titles = []string{
		"Prefix: press " + kit.FormatKeys(m.th.Glyphs, prefix) + ", then",
		"Sessions", "View", "App", "In the sidebar and main view", "In the agent panel and Term", "Dialogs",
	}
	rows = [][]helpRow{
		{{prefix + " " + prefix, "send the prefix key itself to the terminal"}, {"esc", "cancel the prefix"}},
		group("Sessions"), group("View"), group("App"),
		{{"j k / up down", "move in the sidebar"}, {"enter", "open the selected entry"}, {"tab / shift+tab", "next / previous mode"}, {"n", "new session"}, {"/", "command palette"}, {"?", "this help"}, {"q", "detach"}},
		{{"every key", "goes to the terminal; only " + kit.FormatKeys(m.th.Glyphs, prefix) + " is ours"}, {"r", "restart, when the process has exited"}},
		{{"enter", "approve once / run / confirm"}, {"esc", "deny / close"}, {"m", "deny with a message"}, {"ctrl+enter", "start the new session"}, {"tab", "next field"}},
	}
	return
}

func (h *helpOverlay) update(m *Model, msg tea.Msg) (overlay, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return h, nil
	}
	switch k.String() {
	case "esc", "q", "?", "enter":
		return nil, nil
	case "j", "down":
		h.scroll++
	case "k", "up":
		h.scroll = max(h.scroll-1, 0)
	case "pgdown", "space":
		h.scroll += 10
	case "pgup":
		h.scroll = max(h.scroll-10, 0)
	}
	return h, nil
}

func (h *helpOverlay) view(m *Model, screen string) string {
	t := m.th
	mo := kit.ModalOptions{
		Width: 76, Title: "Keys", Meta: "esc close",
		Screen: screen, ScreenWidth: m.w, ScreenHeight: m.h, Top: -1,
	}
	w := kit.ModalInnerWidth(mo)
	titles, rows := m.helpSections()
	lines := []string{
		kit.Fit(paint(t.Text.Secondary, false, "Quitting only detaches: the daemon and your sessions keep running."), w),
		"",
	}
	// The label column starts two cells after the widest key cap.
	keyW := 0
	for _, rs := range rows {
		for _, r := range rs {
			keyW = max(keyW, kit.Width(kit.Keycap(t, r.keys)))
		}
	}
	keyW += 2
	for i, title := range titles {
		if len(rows[i]) == 0 {
			continue
		}
		if len(lines) > 2 {
			lines = append(lines, "")
		}
		lines = append(lines, kit.Fit(paint(t.Text.Muted, true, strings.ToUpper(title)), w))
		for _, r := range rows[i] {
			lines = append(lines, kit.Fit(kit.PadRight(kit.Keycap(t, r.keys), keyW)+paint(t.Text.Secondary, false, r.label), w))
		}
	}
	room := max(m.h-10, 5)
	h.scroll = min(h.scroll, max(len(lines)-room, 0))
	end := min(len(lines), h.scroll+room)
	mo.Body = strings.Join(lines[h.scroll:end], "\n")
	if len(lines) > room {
		mo.Hints = []kit.Hint{{Keys: "up down", Label: "scroll"}}
	}
	return kit.Modal(t, mo)
}
