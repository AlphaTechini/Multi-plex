package gitcli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// SetWorktreeConfig sets a config key for one worktree only. It enables
// extensions.worktreeConfig in the repository if needed and writes the value
// to the worktree's config.worktree under the main repository's .git
// directory, outside the worktree, where files in the worktree cannot alter
// it. It fails if the file would end up inside the worktree.
func (g *Git) SetWorktreeConfig(ctx context.Context, worktree, key, value string) error {
	if err := checkArg("config key", key); err != nil {
		return err
	}
	o := runOpts{dir: worktree}
	on, err := g.runString(ctx, runOpts{dir: worktree, readOnly: true}, "config", "--type=bool", "--default=false", "extensions.worktreeConfig")
	if err != nil {
		return err
	}
	if on != "true" {
		if _, err := g.run(ctx, o, "config", "extensions.worktreeConfig", "true"); err != nil {
			return err
		}
	}
	if _, err := g.run(ctx, o, "config", "--worktree", key, value); err != nil {
		return err
	}
	file, err := g.runString(ctx, runOpts{dir: worktree, readOnly: true}, "rev-parse", "--path-format=absolute", "--git-path", "config.worktree")
	if err != nil {
		return err
	}
	return checkOutside(worktree, filepath.FromSlash(file))
}

// checkOutside returns an error when file lies inside dir.
func checkOutside(dir, file string) error {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	if f, err := filepath.EvalSymlinks(filepath.Dir(file)); err == nil {
		file = filepath.Join(f, filepath.Base(file))
	}
	if rel, err := filepath.Rel(root, file); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("gitcli: worktree config %q is inside the worktree", file)
	}
	return nil
}
