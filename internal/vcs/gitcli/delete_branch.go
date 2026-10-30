package gitcli

import "context"

// DeleteBranch force-deletes a local branch (git branch -D).
func (g *Git) DeleteBranch(ctx context.Context, repo, branch string) error {
	if err := checkArg("branch", branch); err != nil {
		return err
	}
	_, err := g.run(ctx, runOpts{dir: repo}, "branch", "-D", branch)
	return err
}
