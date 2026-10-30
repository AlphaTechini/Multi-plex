package kit

import (
	"image/color"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Tone is the semantic colour of a piece of text, a bar or a pill. Primitives
// take tones instead of colours so every screen maps meaning to colour the
// same way.
type Tone int

// Tones, from neutral text tiers to semantic colours.
const (
	// ToneDefault is primary text.
	ToneDefault Tone = iota
	// ToneStrong is title text.
	ToneStrong
	// ToneSecondary is supporting text.
	ToneSecondary
	// ToneMuted is metadata.
	ToneMuted
	// ToneFaint is placeholder and separator text.
	ToneFaint
	// ToneAccent is the interactive accent.
	ToneAccent
	// ToneInfo is informational (severity info).
	ToneInfo
	// ToneSuccess is a good outcome (state done, added lines).
	ToneSuccess
	// ToneWarning needs attention (state blocked, severity warning).
	ToneWarning
	// ToneDanger is a failure (state error, removed lines).
	ToneDanger
	// ToneDecision needs an answer (severity decision-needed).
	ToneDecision
	// ToneWorking is activity (state working).
	ToneWorking
)

// ToneColor returns the theme colour for a tone.
func ToneColor(t *theme.Theme, tone Tone) color.Color {
	switch tone {
	case ToneStrong:
		return t.Text.Strong
	case ToneSecondary:
		return t.Text.Secondary
	case ToneMuted:
		return t.Text.Muted
	case ToneFaint:
		return t.Text.Faint
	case ToneAccent:
		return t.Accent.Base
	case ToneInfo:
		return t.Severity.Info
	case ToneSuccess:
		return t.State.Done
	case ToneWarning:
		return t.Severity.Warning
	case ToneDanger:
		return t.State.Error
	case ToneDecision:
		return t.Severity.Decision
	case ToneWorking:
		return t.State.Working
	default:
		return t.Text.Primary
	}
}
