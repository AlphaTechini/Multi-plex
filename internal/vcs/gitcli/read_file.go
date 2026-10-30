package gitcli

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ReadFile returns path at rev. An empty rev reads the file from disk; the
// path must stay inside the worktree (no "..", no absolute paths, no symlink
// leading outside). Otherwise it runs git show rev:path.
func (g *Git) ReadFile(ctx context.Context, worktree, rev, p string) ([]byte, error) {
	clean, err := cleanRelPath(p)
	if err != nil {
		return nil, err
	}
	if rev == "" {
		return readInside(worktree, clean)
	}
	if err := checkArg("revision", rev); err != nil {
		return nil, err
	}
	return g.run(ctx, runOpts{dir: worktree, readOnly: true}, "show", rev+":"+clean)
}

// cleanRelPath normalises p to a slash-separated path relative to the root and
// rejects anything that escapes it.
func cleanRelPath(p string) (string, error) {
	slash := filepath.ToSlash(p)
	if slash == "" || strings.ContainsRune(slash, 0) {
		return "", fmt.Errorf("gitcli: invalid path %q", p)
	}
	if strings.HasPrefix(slash, "/") || filepath.IsAbs(p) || filepath.VolumeName(p) != "" {
		return "", fmt.Errorf("gitcli: path %q is outside the worktree", p)
	}
	c := path.Clean(slash)
	if c == "." || c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("gitcli: path %q is outside the worktree", p)
	}
	return c, nil
}

// readInside reads rel (clean, slash-separated) under root, refusing symlinks
// that resolve outside root.
func readInside(root, rel string) ([]byte, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	realFull, err := filepath.EvalSymlinks(full)
	if err != nil {
		return nil, err // includes fs.ErrNotExist
	}
	r, err := filepath.Rel(realRoot, realFull)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("gitcli: path %q is outside the worktree", rel)
	}
	return os.ReadFile(realFull)
}
