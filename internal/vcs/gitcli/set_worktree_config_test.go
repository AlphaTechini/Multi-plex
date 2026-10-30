package gitcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetWorktreeConfig(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt1 := newWorktree(t, g, repo, "c1")
	wt2 := newWorktree(t, g, repo, "c2")

	const key = "remote.origin.pushurl"
	if err := g.SetWorktreeConfig(ctx, wt1, key, "https://example.invalid/push-broker"); err != nil {
		t.Fatal(err)
	}
	if err := g.SetWorktreeConfig(ctx, wt1, key, "https://example.invalid/second"); err != nil { // idempotent, overwrite
		t.Fatal(err)
	}
	if got := gitT(t, wt1, "config", "--get", key); got != "https://example.invalid/second" {
		t.Errorf("wt1 value = %q", got)
	}
	// Other worktrees and the main repo are unaffected.
	for _, dir := range []string{wt2, repo} {
		cmd := gitT(t, dir, "config", "--get", "--default", "unset", key)
		if cmd != "unset" {
			t.Errorf("%s inherited the setting: %q", dir, cmd)
		}
	}
	if gitT(t, repo, "config", "--get", "extensions.worktreeConfig") != "true" {
		t.Error("extensions.worktreeConfig not enabled")
	}
	// The value lives in .git/worktrees/<name>/config.worktree, outside the worktree.
	file := filepath.Join(repo, ".git", "worktrees", "c1", "config.worktree")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("config.worktree: %v", err)
	}
	if !strings.Contains(string(b), "pushurl = https://example.invalid/second") {
		t.Errorf("config.worktree = %q", b)
	}
	if _, err := os.Stat(filepath.Join(wt1, "config.worktree")); err == nil {
		t.Error("config.worktree inside the worktree")
	}
	if err := g.SetWorktreeConfig(ctx, wt1, "--global", "x"); err == nil {
		t.Error("option-like key should be rejected")
	}
}
