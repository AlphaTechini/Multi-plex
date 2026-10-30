package main

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// approval is the biometric approval prompt (docs/04, Layer 5) over the
// Overview screen.
func (s scene) approval() string {
	t := s.t
	mo := kit.ModalOptions{
		Width: 92, Tone: kit.ToneWarning,
		Title: "Approve a change that cannot be undone?",
		Meta:  paint(t.State.Blocked, false, t.Glyphs.Blocked) + " api-pagination",
		Buttons: []kit.ButtonOptions{
			{Label: "Approve once", Keys: "enter", Variant: kit.ButtonPrimary},
			{Label: "Deny", Keys: "esc"},
			{Label: "Deny with message", Keys: "m"},
		},
		Screen: s.overview(), ScreenWidth: s.w, ScreenHeight: s.h, Top: -1,
	}
	iw := kit.ModalInnerWidth(mo)
	mo.Body = s.approvalBody(iw)
	return kit.Modal(t, mo)
}

func (s scene) approvalBody(w int) string {
	t := s.t
	const labelW = 10
	var lines []string
	field := func(label string, vals ...string) {
		for i, v := range vals {
			l := ""
			if i == 0 {
				l = label
			}
			lines = append(lines, kit.Fit(kit.PadRight(paint(t.Text.Muted, false, l), labelW)+v, w))
		}
	}
	lines = append(lines, kit.Fit(paint(t.Text.Secondary, false,
		"The agent wants to delete files outside its worktree. They are not in any checkpoint."), w), "")

	// The command sits on the sunken surface, full width, like a code block.
	cmd := paint(t.Syntax.Function, false, "Remove-Item") + paint(t.Syntax.Keyword, false, " -Recurse -Force ") +
		paint(t.Syntax.String, false, `..\..\multi-plex\testdata\golden`)
	field("Command", kit.Backfill(kit.Fit(" "+cmd, w-labelW), t.Surface.Sunken))
	lines = append(lines, "")
	field("In", paint(t.Text.Primary, false, "api-pagination worktree")+"  "+
		paint(t.Text.Muted, false, kit.TruncateLeft(`%LOCALAPPDATA%\Multiplex\worktrees\multi-plex\api-pagination`, w-labelW-26, t.Glyphs.Ellipsis)))
	field("Why",
		paint(t.Severity.Warning, false, "Layer 2")+paint(t.Text.Primary, false, "  writes outside the session's worktree"),
		paint(t.Severity.Warning, false, "Layer 4")+paint(t.Text.Primary, false, "  recursive delete  ")+paint(t.Text.Muted, false, "rule rm-recursive · .multiplex/rules.toml:14"))
	field("Affects",
		paint(t.Text.Primary, false, "testdata/golden/")+paint(t.Text.Muted, false, "  14 files · 1.2 MB · main checkout"),
		paint(t.Text.Secondary, false, "testdata/golden/session_refresh.json"),
		paint(t.Text.Secondary, false, "testdata/golden/session_expiry.json"),
		paint(t.Text.Muted, false, "+ 12 more  ")+kit.KeyHint(t, kit.Hint{Keys: "l", Label: "list all"}))
	lines = append(lines, "", kit.Divider(t, kit.DividerOptions{Width: w}), "")
	lines = append(lines, kit.Fit(paint(t.Text.Strong, false, "Windows Hello")+paint(t.Text.Secondary, false,
		" will confirm it is you. Approval covers this one command."), w))
	lines = append(lines, kit.Fit(paint(t.Text.Faint, false, "Session-wide approval for this rule is off."), w))
	return strings.Join(lines, "\n")
}
