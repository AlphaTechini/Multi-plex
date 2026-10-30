package core

import "time"

// Severity of an insight note.
type Severity string

// Note severities.
const (
	SeverityInfo           Severity = "info"
	SeverityWarning        Severity = "warning"
	SeverityDecisionNeeded Severity = "decision-needed"
)

// Note is a numbered "you should know" card an agent created (note_to_user).
type Note struct {
	ID        string    `json:"id"`
	SessionID SessionID `json:"session_id"`
	Number    int       `json:"number"` // 1-based, per session
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Severity  Severity  `json:"severity"`
	Refs      []string  `json:"refs"` // "path:L40-L52"
	Dismissed bool      `json:"dismissed"`
	At        time.Time `json:"at"`
}

// NoteAction is what the user does with a card.
type NoteAction string

// Note actions. See docs/08-insights-and-steering.md.
const (
	NoteAsk            NoteAction = "ask"
	NoteSteer          NoteAction = "steer"
	NoteInterruptSteer NoteAction = "interrupt-steer"
	NoteDismiss        NoteAction = "dismiss"
)
