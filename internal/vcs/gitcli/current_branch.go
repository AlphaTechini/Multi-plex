package gitcli

import "context"

// CurrentBranch returns the checked-out branch name in dir. On a detached HEAD
// it returns "detached@" followed by the abbreviated commit hash, with a nil
// error, so the result is never empty and is not a valid branch name.
func (g *Git) CurrentBranch(ctx context.Context, dir string) (string, error) {
	o := runOpts{dir: dir, readOnly: true}
	out, err := g.runString(ctx, o, "symbolic-ref", "--short", "-q", "HEAD")
	if err == nil {
		return out, nil
	}
	if exitCode(err) != 1 { // 1 with -q means HEAD is not a symbolic ref
		return "", err
	}
	short, err := g.runString(ctx, o, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return "detached@" + short, nil
}
