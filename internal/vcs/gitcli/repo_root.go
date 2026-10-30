package gitcli

import (
	"context"
	"path/filepath"
)

// RepoRoot returns the top-level directory of the repository containing dir,
// as an OS path (git prints forward slashes on Windows).
func (g *Git) RepoRoot(ctx context.Context, dir string) (string, error) {
	out, err := g.runString(ctx, runOpts{dir: dir, readOnly: true}, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.FromSlash(out)), nil
}
