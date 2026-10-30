package gitcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var ctx = context.Background()

// needGit skips the test when git is missing and isolates it from the user's
// git configuration.
func needGit(t *testing.T) *Git {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return New()
}

// gitT runs git in dir and returns trimmed stdout, failing the test on error.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo creates a repository on branch main with one commit containing
// README.md, and a local identity.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := realTemp(t)
	gitT(t, dir, "init", "-q", "-b", "main")
	gitT(t, dir, "config", "user.name", "Test")
	gitT(t, dir, "config", "user.email", "test@example.com")
	write(t, dir, "README.md", "hello\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

// realTemp returns a temp dir with symlinks resolved (macOS /var -> /private/var).
func realTemp(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", msg)
}

// newWorktree adds a worktree on a new branch from main and returns its path.
func newWorktree(t *testing.T, g *Git, repo, name string) string {
	t.Helper()
	wt := filepath.Join(realTemp(t), "wt", name)
	if err := g.WorktreeAdd(ctx, repo, wt, "mpx/"+name, "main"); err != nil {
		t.Fatal(err)
	}
	return wt
}

func indexState(t *testing.T, dir string) string {
	t.Helper()
	return gitT(t, dir, "ls-files", "-s") + "\n--\n" + gitT(t, dir, "diff", "--cached", "--name-only") + "\n--\n" + gitT(t, dir, "rev-parse", "HEAD")
}
