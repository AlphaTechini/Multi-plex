package gitcli

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Restore resets the worktree's files to a checkpoint commit: tracked files
// are rewritten from it, and files that are now present (tracked or untracked)
// but were not in the checkpoint are deleted. Ignored files, nested
// repositories, HEAD, the branch and the index are untouched.
func (g *Git) Restore(ctx context.Context, worktree, commit string) error {
	if err := checkArg("commit", commit); err != nil {
		return err
	}
	o := runOpts{dir: worktree}
	root, err := g.RepoRoot(ctx, worktree)
	if err != nil {
		return err
	}
	o.dir = root
	full, err := g.runString(ctx, runOpts{dir: root, readOnly: true}, "rev-parse", "--verify", commit+"^{commit}")
	if err != nil {
		return err
	}
	want, err := g.pathSet(ctx, runOpts{dir: root, readOnly: true}, "ls-tree", "-r", "-z", "--name-only", full)
	if err != nil {
		return err
	}
	have, err := g.run(ctx, runOpts{dir: root, readOnly: true}, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return err
	}
	// Delete extras first so a file can be replaced by a directory of the same name.
	for _, b := range bytes.Split(have, []byte{0}) {
		p := string(b)
		if p == "" || strings.HasSuffix(p, "/") || want[p] { // "dir/" is a nested repository
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.Remove(abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		removeEmptyParents(root, filepath.Dir(abs))
	}
	_, err = g.run(ctx, o, "restore", "--source="+full, "--worktree", "--", ":/")
	return err
}

// pathSet runs a -z listing command and returns its entries as a set.
func (g *Git) pathSet(ctx context.Context, o runOpts, args ...string) (map[string]bool, error) {
	out, err := g.run(ctx, o, args...)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, b := range bytes.Split(out, []byte{0}) {
		if len(b) > 0 {
			set[string(b)] = true
		}
	}
	return set, nil
}

// removeEmptyParents removes dir and its parents while they are empty, never
// going above root.
func removeEmptyParents(root, dir string) {
	for dir != root && strings.HasPrefix(dir, root) {
		if os.Remove(dir) != nil { // not empty (or gone)
			return
		}
		dir = filepath.Dir(dir)
	}
}
