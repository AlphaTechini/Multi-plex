package core

import "time"

// Summary is an agent's structured progress report (report_progress).
type Summary struct {
	SessionID     SessionID `json:"session_id"`
	Done          []string  `json:"done"`
	FilesChanged  []string  `json:"files_changed"`
	Decisions     []string  `json:"decisions"`
	OpenQuestions []string  `json:"open_questions"`
	Next          string    `json:"next"`
	At            time.Time `json:"at"`
}
