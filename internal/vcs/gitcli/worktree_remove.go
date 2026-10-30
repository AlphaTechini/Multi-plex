package gitcli

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

const (
	removeAttempts = 10
	removeMinDelay = 100 * time.Millisecond
	removeMaxDelay = time.Second
)

// WorktreeRemove removes a worktree (git worktree remove --force). While files
// are busy (Windows antivirus scanners and open handles) it retries up to ten
// times with backoff from 100ms to 1s, then prunes stale worktree records. A
// worktree directory that is already gone is not an error.
func (g *Git) WorktreeRemove(ctx context.Context, repo, path string) error {
	if err := checkArg("worktree path", path); err != nil {
		return err
	}
	o := runOpts{dir: repo}
	err := retry(ctx, removeAttempts, removeMinDelay, removeMaxDelay, func() (bool, error) {
		_, err := g.run(ctx, o, "worktree", "remove", "--force", "--", path)
		if err == nil {
			return false, nil
		}
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return false, nil // gone already; prune below clears the record
		}
		return isBusy(err), err
	})
	_, pruneErr := g.run(context.WithoutCancel(ctx), o, "worktree", "prune")
	if err != nil {
		return err
	}
	return pruneErr
}

// retry calls fn up to attempts times, sleeping between tries with a delay that
// doubles from min up to max. fn returns whether a failure is worth retrying.
// The last error is returned; cancelling ctx stops the wait.
func retry(ctx context.Context, attempts int, min, max time.Duration, fn func() (retryable bool, err error)) error {
	delay := min
	var err error
	for i := 0; i < attempts; i++ {
		var again bool
		if again, err = fn(); err == nil || !again {
			return err
		}
		if i == attempts-1 {
			break
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		if delay *= 2; delay > max {
			delay = max
		}
	}
	return err
}

// busyMarkers are fragments of git/OS messages meaning "a file is in use".
var busyMarkers = []string{
	"permission denied", "access is denied", "directory not empty",
	"used by another process", "device or resource busy", "resource busy",
	"failed to delete", "unable to unlink", "could not remove", "cannot remove",
}

// isBusy reports whether err looks like a transient file-in-use failure.
func isBusy(err error) bool {
	var ge *Error
	if !errors.As(err, &ge) {
		return false
	}
	msg := strings.ToLower(ge.Stderr)
	for _, m := range busyMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}
