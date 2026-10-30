package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Styles are lipgloss styles derived from a theme's tokens, for code that
// renders with lipgloss directly. The kit package covers most needs; use
// these for one-off text inside kit primitives. None of them set a
// background, so they inherit the surface they are drawn on.
type Styles struct {
	// Title is a region or dialog title: strong and bold.
	Title lipgloss.Style
	// Body is primary text.
	Body lipgloss.Style
	// Secondary is supporting text.
	Secondary lipgloss.Style
	// Muted is metadata and labels.
	Muted lipgloss.Style
	// Faint is placeholders and separators.
	Faint lipgloss.Style
	// Label is an uppercase section label (SESSIONS, SUMMARY): muted and bold.
	Label lipgloss.Style
	// Key is a key name inside a key hint.
	Key lipgloss.Style
	// KeyLabel is the action text after a key name.
	KeyLabel lipgloss.Style
	// Link is a path or reference that can be opened.
	Link lipgloss.Style
	// Code is inline code and paths in prose.
	Code lipgloss.Style
	// Working, Blocked, Done, Idle, Error and Starting colour state text.
	Working, Blocked, Done, Idle, Error, Starting lipgloss.Style
	// Info, Warning and Decision colour severity text.
	Info, Warning, Decision lipgloss.Style
	// Added and Removed colour diff counts and signs.
	Added, Removed lipgloss.Style
}

// NewStyles derives the lipgloss styles from t's tokens.
func NewStyles(t *Theme) Styles {
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	return Styles{
		Title:     fg(t.Text.Strong).Bold(true),
		Body:      fg(t.Text.Primary),
		Secondary: fg(t.Text.Secondary),
		Muted:     fg(t.Text.Muted),
		Faint:     fg(t.Text.Faint),
		Label:     fg(t.Text.Muted).Bold(true),
		Key:       fg(t.Text.Strong).Bold(true),
		KeyLabel:  fg(t.Text.Muted),
		Link:      fg(t.Accent.Base).Underline(true),
		Code:      fg(t.Syntax.Variable),
		Working:   fg(t.State.Working),
		Blocked:   fg(t.State.Blocked),
		Done:      fg(t.State.Done),
		Idle:      fg(t.State.Idle),
		Error:     fg(t.State.Error),
		Starting:  fg(t.State.Starting),
		Info:      fg(t.Severity.Info),
		Warning:   fg(t.Severity.Warning),
		Decision:  fg(t.Severity.Decision),
		Added:     fg(t.Diff.AddSign),
		Removed:   fg(t.Diff.DelSign),
	}
}
