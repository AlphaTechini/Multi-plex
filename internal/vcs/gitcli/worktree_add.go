package gitcli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// WorktreeAdd creates a worktree at path on a new branch started from base.
// Missing parent directories of path are created. On Windows core.longpaths is
// enabled in the repository first.
func (g *Git) WorktreeAdd(ctx context.Context, repo, path, branch, base string) error {
	if err := checkArg("branch", branch); err != nil {
		return err
	}
	if err := checkArg("base", base); err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("gitcli: empty worktree path")
	}
	abs, err := filepath.Abs(path) // git runs in repo, so a relative path would move
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	o := runOpts{dir: repo}
	if runtime.GOOS == "windows" {
		if _, err := g.run(ctx, o, "config", "core.longpaths", "true"); err != nil {
			return err
		}
	}
	_, err = g.run(ctx, o, "worktree", "add", "-b", branch, "--", abs, base)
	return err
}
