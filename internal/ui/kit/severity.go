package kit

import (
	"image/color"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Severity is an insight card severity (docs/08).
type Severity int

// Card severities.
const (
	// SeverityInfo needs no answer.
	SeverityInfo Severity = iota
	// SeverityWarning flags a risk.
	SeverityWarning
	// SeverityDecision needs an answer.
	SeverityDecision
)

// Label returns the upper-case label shown on cards.
func (s Severity) Label() string {
	switch s {
	case SeverityWarning:
		return "WARNING"
	case SeverityDecision:
		return "DECISION NEEDED"
	default:
		return "INFO"
	}
}

// Color returns the severity's colour in t.
func (s Severity) Color(t *theme.Theme) color.Color {
	switch s {
	case SeverityWarning:
		return t.Severity.Warning
	case SeverityDecision:
		return t.Severity.Decision
	default:
		return t.Severity.Info
	}
}
