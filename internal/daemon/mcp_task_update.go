package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/spec"
)

// mcpTaskUpdate changes a task's checkbox in the session's worktree copy of
// tasks.md, so the change is reviewable in the diff. A note is recorded as a
// nested bullet under the task. "blocked" keeps the box open and requires a
// note, since a checkbox has no blocked marker.
func (d *Daemon) mcpTaskUpdate(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.TaskUpdateArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	state, note := spec.TaskOpen, clipText(a.Note, maxItem)
	switch a.Status {
	case "open":
	case "in-progress":
		state = spec.TaskInProgress
	case "done":
		state = spec.TaskDone
	case "blocked":
		if note == "" {
			return "", badRequest("a note saying what blocks the task is required")
		}
		note = "BLOCKED: " + note
	default:
		return "", badRequest("status must be open, in-progress, done or blocked, got %q", a.Status)
	}

	doc, err := d.readSpec(ctx, s, a.Slug)
	if err != nil {
		return "", err
	}
	// The worktree's tasks.md is the one that changes; fall back to the
	// workspace copy as the starting text if the worktree has none yet.
	path := specFile(s.snap().WorktreePath, a.Slug, "tasks.md")
	md := doc.Tasks
	updated, err := spec.UpdateTask(md, a.TaskID, state)
	if err != nil {
		switch {
		case errors.Is(err, spec.ErrTaskNotFound):
			return "", notFound("no task %q in spec %q", a.TaskID, a.Slug)
		case errors.Is(err, spec.ErrAmbiguousTask):
			return "", badRequest("%v", err)
		}
		return "", err
	}
	if note != "" {
		updated = insertTaskNote(updated, a.TaskID, note)
	}
	if err := writeFileAtomic(path, []byte(updated)); err != nil {
		return "", fmt.Errorf("write tasks.md: %w", err)
	}
	d.scheduleDiffChanged(s)
	return fmt.Sprintf("task %s is now %s", a.TaskID, a.Status), nil
}

// insertTaskNote adds "  - Note: text" under the task's line. ParseTasks
// does not treat it as a task.
func insertTaskNote(md, taskID, note string) string {
	for _, t := range spec.ParseTasks(md) {
		if t.ID != taskID {
			continue
		}
		lines := strings.Split(md, "\n")
		line := lines[t.Line-1]
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		bullet := indent + "  - Note: " + strings.ReplaceAll(note, "\n", " ")
		lines = append(lines[:t.Line], append([]string{bullet}, lines[t.Line:]...)...)
		return strings.Join(lines, "\n")
	}
	return md
}
