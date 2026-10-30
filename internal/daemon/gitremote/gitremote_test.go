package gitremote

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestHasRemoteAndPush(t *testing.T) {
	root := t.TempDir()
	repo, origin := filepath.Join(root, "repo"), filepath.Join(root, "origin.git")
	git(t, root, "init", "--bare", "-b", "main", origin)
	git(t, root, "init", "-b", "main", repo)
	if err := os.WriteFile(filepath.Join(repo, "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "a")

	g := Git{}
	ctx := context.Background()
	if g.HasRemote(ctx, repo, "origin") {
		t.Error("HasRemote true before adding a remote")
	}
	git(t, repo, "remote", "add", "origin", origin)
	if !g.HasRemote(ctx, repo, "origin") {
		t.Error("HasRemote false after adding origin")
	}
	if g.HasRemote(ctx, repo, "-x") || g.HasRemote(ctx, repo, "") {
		t.Error("HasRemote accepted an option-like name")
	}

	out, err := g.Push(ctx, repo, "origin", "main")
	if err != nil {
		t.Fatalf("push: %v\n%s", err, out)
	}
	if got := git(t, origin, "rev-parse", "main"); got != git(t, repo, "rev-parse", "main") {
		t.Errorf("remote main = %s", got)
	}
	if _, err := g.Push(ctx, repo, "origin", "no-such-branch"); err == nil {
		t.Error("pushing a missing branch succeeded")
	}
	if _, err := g.Push(ctx, repo, "--force", "main"); err == nil {
		t.Error("an option-like remote was accepted")
	}
}
