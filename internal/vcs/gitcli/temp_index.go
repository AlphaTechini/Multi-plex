package gitcli

import (
	"context"
	"os"
	"path/filepath"
)

// withTempIndex builds a private index holding HEAD plus every change in the
// worktree (modified, deleted and untracked non-ignored files), then calls fn
// with the environment that makes git use it. The worktree's real index is
// never read or written, so an agent running git at the same time is not
// disturbed. The temporary index is removed afterwards.
func (g *Git) withTempIndex(ctx context.Context, worktree string, fn func(o runOpts) error) error {
	dir, err := os.MkdirTemp("", "mpx-index-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	o := runOpts{dir: worktree, env: []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}}
	if _, err := g.run(ctx, o, "read-tree", "HEAD"); err != nil {
		return err
	}
	if _, err := g.run(ctx, o, "add", "-A"); err != nil {
		return err
	}
	return fn(o)
}

// diffBase returns the commit the session's changes should be measured from:
// the merge base of base and HEAD, so commits that landed on base after the
// worktree branched are not shown as session changes. When the two share no
// history, the resolved base itself is used.
func (g *Git) diffBase(ctx context.Context, worktree, base string) (string, error) {
	if err := checkArg("base", base); err != nil {
		return "", err
	}
	o := runOpts{dir: worktree, readOnly: true}
	mb, err := g.runString(ctx, o, "merge-base", base, "HEAD")
	if err == nil && mb != "" {
		return mb, nil
	}
	if err != nil && exitCode(err) != 1 { // 1 means no common ancestor
		return "", err
	}
	return g.runString(ctx, o, "rev-parse", "--verify", base+"^{commit}")
}
