package memory

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// Copy helpers. Slices are copied (nil stays nil, empty stays empty) and
// times are normalised to UTC so every implementation returns the same values.

func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	return append([]string{}, s...)
}

func utc(t time.Time) time.Time { return t.Round(0).UTC() }

func cloneWorkspace(w core.Workspace) core.Workspace {
	w.CreatedAt = utc(w.CreatedAt)
	return w
}

func cloneSession(s core.Session) core.Session {
	s.CreatedAt = utc(s.CreatedAt)
	s.UpdatedAt = utc(s.UpdatedAt)
	return s
}

func cloneSummary(s core.Summary) core.Summary {
	s.Done = cloneStrings(s.Done)
	s.FilesChanged = cloneStrings(s.FilesChanged)
	s.Decisions = cloneStrings(s.Decisions)
	s.OpenQuestions = cloneStrings(s.OpenQuestions)
	s.At = utc(s.At)
	return s
}

func cloneNote(n core.Note) core.Note {
	n.Refs = cloneStrings(n.Refs)
	n.At = utc(n.At)
	return n
}

func cloneComment(c core.Comment) core.Comment {
	c.At = utc(c.At)
	return c
}

func cloneCheckpoint(c core.Checkpoint) core.Checkpoint {
	c.At = utc(c.At)
	return c
}

func cloneAudit(e store.AuditEntry) store.AuditEntry {
	e.Request.Paths = cloneStrings(e.Request.Paths)
	e.Request.At = utc(e.Request.At)
	e.Decision.At = utc(e.Decision.At)
	return e
}
