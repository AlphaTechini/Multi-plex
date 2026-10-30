package spec

import (
	"fmt"
	"strings"
)

// TaskState is the state UpdateTask writes into a checkbox.
type TaskState string

const (
	// TaskOpen writes "[ ]".
	TaskOpen TaskState = "open"
	// TaskDone writes "[x]".
	TaskDone TaskState = "done"
	// TaskInProgress writes "[~]".
	TaskInProgress TaskState = "in-progress"
)

// marker returns the checkbox character for a state.
func (s TaskState) marker() (byte, bool) {
	switch s {
	case TaskOpen:
		return ' ', true
	case TaskDone:
		return 'x', true
	case TaskInProgress:
		return '~', true
	}
	return 0, false
}

// UpdateTask returns md with the checkbox of the task with the given ID set to
// state. Only that one marker character changes; the rest of the file is kept
// byte for byte. It returns ErrTaskNotFound if no task has the ID, and
// ErrAmbiguousTask if several do.
func UpdateTask(md string, id string, state TaskState) (string, error) {
	marker, ok := state.marker()
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrInvalidState, state)
	}

	var match *Task
	tasks := ParseTasks(md)
	for i := range tasks {
		if tasks[i].ID != id {
			continue
		}
		if match != nil {
			return "", fmt.Errorf("%w: %q", ErrAmbiguousTask, id)
		}
		match = &tasks[i]
	}
	if match == nil {
		return "", fmt.Errorf("%w: %q", ErrTaskNotFound, id)
	}

	lines := strings.Split(md, "\n")
	line := lines[match.Line-1]
	loc := taskRe.FindStringSubmatchIndex(line)
	// loc[4:6] is the byte range of the marker character (group 2).
	lines[match.Line-1] = line[:loc[4]] + string(marker) + line[loc[5]:]
	return strings.Join(lines, "\n"), nil
}
