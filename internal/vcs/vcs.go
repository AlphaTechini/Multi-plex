// Package vcs is the neutral version-control layer. Worktrees, diffs,
// checkpoints and conflict prediction go through the VCS interface; the git
// command-line implementation lives in a subpackage.
package vcs

import "context"

// FileStatus is how a file changed.
type FileStatus string

// File statuses.
const (
	Added    FileStatus = "added"
	Modified FileStatus = "modified"
	Deleted  FileStatus = "deleted"
	Renamed  FileStatus = "renamed"
)

// FileChange is one changed file in a worktree compared with its base.
type FileChange struct {
	Path    string     `json:"path"`     // new path, slash-separated, relative to the worktree root
	OldPath string     `json:"old_path"` // for renames
	Status  FileStatus `json:"status"`
	Added   int        `json:"added"`
	Deleted int        `json:"deleted"`
	Binary  bool       `json:"binary"`
}

// VCS is everything Multiplex needs from version control.
type VCS interface {
	// RepoRoot returns the top-level directory of the repository containing dir.
	RepoRoot(ctx context.Context, dir string) (string, error)
	// CurrentBranch returns the checked-out branch name in dir.
	CurrentBranch(ctx context.Context, dir string) (string, error)
	// ResolveRev returns the full commit hash of rev in dir.
	ResolveRev(ctx context.Context, dir, rev string) (string, error)
	// WorktreeAdd creates a worktree at path on a new branch started from base.
	WorktreeAdd(ctx context.Context, repo, path, branch, base string) error
	// WorktreeRemove removes a worktree, retrying while files are busy (Windows).
	WorktreeRemove(ctx context.Context, repo, path string) error
	// DeleteBranch deletes a local branch.
	DeleteBranch(ctx context.Context, repo, branch string) error
	// Diff returns a unified diff (histogram algorithm) of the worktree,
	// including uncommitted and untracked files, against base.
	Diff(ctx context.Context, worktree, base string) (string, error)
	// ChangedFiles lists changed files of the worktree against base,
	// including untracked files.
	ChangedFiles(ctx context.Context, worktree, base string) ([]FileChange, error)
	// ReadFile returns path at rev ("" means the working tree).
	ReadFile(ctx context.Context, worktree, rev, path string) ([]byte, error)
	// MergeConflicts returns files that would conflict if a and b were
	// merged, without touching any worktree (git merge-tree --write-tree).
	MergeConflicts(ctx context.Context, repo, a, b string) ([]string, error)
	// Checkpoint snapshots tracked and untracked (non-ignored) files of the
	// worktree into a commit stored under ref, using a temporary index so
	// the staging area is untouched. Returns the commit hash.
	Checkpoint(ctx context.Context, worktree, ref, message string) (string, error)
	// Restore resets the worktree's files to a checkpoint commit.
	Restore(ctx context.Context, worktree, commit string) error
	// SetWorktreeConfig sets a config key for one worktree only
	// (extensions.worktreeConfig), stored outside the worktree.
	SetWorktreeConfig(ctx context.Context, worktree, key, value string) error
	// Version returns the VCS tool version string.
	Version(ctx context.Context) (string, error)
}
