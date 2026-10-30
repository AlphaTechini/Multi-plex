package spec

import "errors"

var (
	// ErrInvalidSlug is returned for a slug that contains a path separator, a colon or "..".
	ErrInvalidSlug = errors.New("spec: invalid slug")
	// ErrTaskNotFound is returned by UpdateTask when no task has the given ID.
	ErrTaskNotFound = errors.New("spec: task not found")
	// ErrAmbiguousTask is returned by UpdateTask when more than one task has the given ID.
	ErrAmbiguousTask = errors.New("spec: task id is ambiguous")
	// ErrInvalidState is returned by UpdateTask for an unknown TaskState.
	ErrInvalidState = errors.New("spec: invalid task state")
)
