package gitcli

import (
	"context"
	"fmt"
	"strings"
)

// checkpointIdentity lets commit-tree work when the user has no git identity.
var checkpointIdentity = []string{
	"GIT_AUTHOR_NAME=multiplex", "GIT_AUTHOR_EMAIL=multiplex@localhost",
	"GIT_COMMITTER_NAME=multiplex", "GIT_COMMITTER_EMAIL=multiplex@localhost",
}

// Checkpoint snapshots the worktree (tracked and untracked non-ignored files)
// into a commit whose parent is HEAD and points ref at it. A temporary index
// is used, so the real index, HEAD, the branch and working files are
// unchanged. It returns the commit hash.
func (g *Git) Checkpoint(ctx context.Context, worktree, ref, message string) (string, error) {
	if !strings.HasPrefix(ref, "refs/") {
		return "", fmt.Errorf("gitcli: checkpoint ref %q must start with refs/", ref)
	}
	if message == "" {
		message = "multiplex checkpoint"
	}
	var commit string
	err := g.withTempIndex(ctx, worktree, func(o runOpts) error {
		tree, err := g.runString(ctx, o, "write-tree")
		if err != nil {
			return err
		}
		o.env = append(o.env, checkpointIdentity...)
		commit, err = g.runString(ctx, o, "commit-tree", tree, "-p", "HEAD", "-m", message)
		return err
	})
	if err != nil {
		return "", err
	}
	if _, err := g.run(ctx, runOpts{dir: worktree}, "update-ref", ref, commit); err != nil {
		return "", err
	}
	return commit, nil
}
