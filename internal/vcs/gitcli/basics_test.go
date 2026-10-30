package gitcli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoRoot(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	write(t, repo, "sub dir/x.txt", "x")
	for _, dir := range []string{repo, filepath.Join(repo, "sub dir")} {
		got, err := g.RepoRoot(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != repo {
			t.Errorf("RepoRoot(%q) = %q, want %q", dir, got, repo)
		}
	}
	if _, err := g.RepoRoot(ctx, realTemp(t)); err == nil {
		t.Error("expected error outside a repository")
	} else if !strings.Contains(err.Error(), "git rev-parse") {
		t.Errorf("error should name the subcommand: %v", err)
	}
}

func TestCurrentBranch(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	got, err := g.CurrentBranch(ctx, repo)
	if err != nil || got != "main" {
		t.Fatalf("CurrentBranch = %q, %v", got, err)
	}
	gitT(t, repo, "checkout", "-q", "--detach")
	got, err = g.CurrentBranch(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	short := gitT(t, repo, "rev-parse", "--short", "HEAD")
	if got != "detached@"+short {
		t.Errorf("detached = %q, want detached@%s", got, short)
	}
	if _, err := g.CurrentBranch(ctx, realTemp(t)); err == nil {
		t.Error("expected error outside a repository")
	}
}

func TestResolveRev(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	want := gitT(t, repo, "rev-parse", "HEAD")
	gitT(t, repo, "tag", "-a", "-m", "t", "v1") // annotated tag must peel to the commit
	for _, rev := range []string{"HEAD", "main", "v1", want} {
		got, err := g.ResolveRev(ctx, repo, rev)
		if err != nil || got != want {
			t.Errorf("ResolveRev(%q) = %q, %v; want %q", rev, got, err, want)
		}
	}
	for _, rev := range []string{"nope", "", "--all", "-x"} {
		if _, err := g.ResolveRev(ctx, repo, rev); err == nil {
			t.Errorf("ResolveRev(%q) should fail", rev)
		}
	}
}

func TestVersion(t *testing.T) {
	g := needGit(t)
	v, err := g.Version(ctx)
	if err != nil || !strings.HasPrefix(v, "git version ") {
		t.Fatalf("Version = %q, %v", v, err)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in       string
		maj, min int
		bad      bool
	}{
		{"git version 2.43.0", 2, 43, false},
		{"git version 2.39.2.windows.1", 2, 39, false},
		{"git version 2.37.1 (Apple Git-137.1)", 2, 37, false},
		{"git version 10.1", 10, 1, false},
		{"garbage", 0, 0, true},
		{"git version x.y", 0, 0, true},
	}
	for _, tt := range tests {
		maj, min, err := parseVersion(tt.in)
		if (err != nil) != tt.bad || maj != tt.maj || min != tt.min {
			t.Errorf("parseVersion(%q) = %d,%d,%v", tt.in, maj, min, err)
		}
	}
}

func TestRunCancel(t *testing.T) {
	g := needGit(t)
	c, cancel := context.WithCancel(ctx)
	cancel()
	_, err := g.Version(c)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRunMissingExecutable(t *testing.T) {
	needGit(t)
	g := NewWithPath(filepath.Join(t.TempDir(), "no-such-git"))
	if _, err := g.Version(ctx); err == nil {
		t.Error("expected error for missing executable")
	}
}

func TestSubcommand(t *testing.T) {
	tests := map[string][]string{
		"diff":       {"-c", "core.quotepath=false", "diff", "--cached"},
		"worktree":   {"-C", "x", "worktree", "add"},
		"rev-parse":  {"rev-parse", "--show-toplevel"},
		"git":        {},
		"merge-tree": {"merge-tree", "--write-tree"},
	}
	for want, args := range tests {
		if got := subcommand(args); got != want {
			t.Errorf("subcommand(%v) = %q, want %q", args, got, want)
		}
	}
}
