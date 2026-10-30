package gitcli

import "context"

// diffFlags make output stable regardless of the user's git configuration.
var diffFlags = []string{
	"-c", "core.quotepath=false",
	"diff", "--cached", "--histogram", "--no-color", "--no-ext-diff", "--no-textconv",
	"-M", "--src-prefix=a/", "--dst-prefix=b/",
}

// Diff returns a unified diff (histogram algorithm, rename detection) of the
// worktree against the merge base of base and HEAD. It includes uncommitted
// changes and untracked non-ignored files (shown as additions). The worktree's
// index is not touched.
func (g *Git) Diff(ctx context.Context, worktree, base string) (string, error) {
	mb, err := g.diffBase(ctx, worktree, base)
	if err != nil {
		return "", err
	}
	var out []byte
	err = g.withTempIndex(ctx, worktree, func(o runOpts) error {
		o.readOnly = true
		args := append(append([]string(nil), diffFlags...), mb)
		out, err = g.run(ctx, o, args...)
		return err
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}
