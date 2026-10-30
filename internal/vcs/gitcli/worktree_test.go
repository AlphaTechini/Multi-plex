package gitcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorktreeAddRemoveDeleteBranch(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := filepath.Join(realTemp(t), "deep", "er", "work tree") // missing parents, space in path
	if err := g.WorktreeAdd(ctx, repo, wt, "mpx/one", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt, "README.md")); err != nil {
		t.Fatalf("worktree not populated: %v", err)
	}
	if got, _ := g.CurrentBranch(ctx, wt); got != "mpx/one" {
		t.Errorf("branch = %q", got)
	}
	if err := g.WorktreeAdd(ctx, repo, filepath.Join(wt, "x"), "mpx/one", "main"); err == nil {
		t.Error("adding an existing branch should fail")
	}
	for _, bad := range []struct{ branch, base string }{{"-x", "main"}, {"b", "--detach"}, {"", "main"}} {
		if err := g.WorktreeAdd(ctx, repo, wt+"2", bad.branch, bad.base); err == nil {
			t.Errorf("WorktreeAdd(%q,%q) should fail", bad.branch, bad.base)
		}
	}

	// A dirty worktree (untracked + modified) is removed because of --force.
	write(t, wt, "scratch.txt", "x")
	write(t, wt, "README.md", "changed\n")
	if err := g.WorktreeRemove(ctx, repo, wt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists: %v", err)
	}
	if list := gitT(t, repo, "worktree", "list", "--porcelain"); strings.Contains(list, "work tree") {
		t.Errorf("worktree still registered:\n%s", list)
	}
	if err := g.DeleteBranch(ctx, repo, "mpx/one"); err != nil {
		t.Fatal(err)
	}
	if gitT(t, repo, "branch", "--list", "mpx/one") != "" {
		t.Error("branch still exists")
	}
	if err := g.DeleteBranch(ctx, repo, "mpx/one"); err == nil {
		t.Error("deleting a missing branch should fail")
	}
	if err := g.DeleteBranch(ctx, repo, "-D"); err == nil {
		t.Error("option-like branch should be rejected")
	}
}

func TestWorktreeRemoveAlreadyGone(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "gone")
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if err := g.WorktreeRemove(ctx, repo, wt); err != nil {
		t.Fatalf("removing a deleted worktree: %v", err)
	}
	if list := gitT(t, repo, "worktree", "list", "--porcelain"); strings.Contains(list, "gone") {
		t.Errorf("stale record not pruned:\n%s", list)
	}
}

func TestWorktreeRemoveNotAWorktree(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	other := realTemp(t)
	write(t, other, "keep.txt", "k")
	if err := g.WorktreeRemove(ctx, repo, other); err == nil {
		t.Error("expected error removing a non-worktree")
	}
	if _, err := os.Stat(filepath.Join(other, "keep.txt")); err != nil {
		t.Error("a non-worktree directory must never be deleted")
	}
}

func TestRetry(t *testing.T) {
	boom := errors.New("busy")
	t.Run("succeeds after retries", func(t *testing.T) {
		n := 0
		err := retry(ctx, 10, time.Millisecond, 4*time.Millisecond, func() (bool, error) {
			if n++; n < 4 {
				return true, boom
			}
			return false, nil
		})
		if err != nil || n != 4 {
			t.Errorf("err=%v n=%d", err, n)
		}
	})
	t.Run("gives up", func(t *testing.T) {
		n := 0
		err := retry(ctx, 5, time.Millisecond, time.Millisecond, func() (bool, error) { n++; return true, boom })
		if err != boom || n != 5 {
			t.Errorf("err=%v n=%d", err, n)
		}
	})
	t.Run("non-retryable stops at once", func(t *testing.T) {
		n := 0
		err := retry(ctx, 5, time.Millisecond, time.Millisecond, func() (bool, error) { n++; return false, boom })
		if err != boom || n != 1 {
			t.Errorf("err=%v n=%d", err, n)
		}
	})
	t.Run("context cancel", func(t *testing.T) {
		c, cancel := context.WithCancel(ctx)
		err := retry(c, 10, time.Hour, time.Hour, func() (bool, error) { cancel(); return true, boom })
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err=%v", err)
		}
	})
}

func TestIsBusy(t *testing.T) {
	tests := []struct {
		stderr string
		want   bool
	}{
		{"error: failed to delete 'C:/x/node_modules': Directory not empty", true},
		{"error: failed to delete 'x': Permission denied", true},
		{"error: The process cannot access the file because it is being used by another process.", true},
		{"fatal: 'x' is not a working tree", false},
		{"fatal: cannot lock ref", false},
		{"Access is denied.", true},
	}
	for _, tt := range tests {
		if got := isBusy(&Error{Subcommand: "worktree", Stderr: tt.stderr}); got != tt.want {
			t.Errorf("isBusy(%q) = %v", tt.stderr, got)
		}
	}
	if isBusy(errors.New("x")) {
		t.Error("non-git errors are not busy")
	}
}
