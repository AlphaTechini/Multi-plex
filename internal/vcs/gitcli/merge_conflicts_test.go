package gitcli

import (
	"reflect"
	"testing"
)

func TestMergeConflicts(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	write(t, repo, "a b.txt", "base\n")
	write(t, repo, "c.txt", "base\n")
	write(t, repo, "free.txt", "base\n")
	commitAll(t, repo, "files")
	gitT(t, repo, "checkout", "-q", "-b", "left")
	write(t, repo, "a b.txt", "left\n")
	write(t, repo, "c.txt", "left\n")
	write(t, repo, "left-only.txt", "l\n")
	commitAll(t, repo, "left")
	gitT(t, repo, "checkout", "-q", "main")
	gitT(t, repo, "checkout", "-q", "-b", "right")
	write(t, repo, "a b.txt", "right\n")
	write(t, repo, "c.txt", "right\n")
	write(t, repo, "free.txt", "right\n")
	commitAll(t, repo, "right")
	gitT(t, repo, "checkout", "-q", "main")
	gitT(t, repo, "checkout", "-q", "-b", "clean")
	write(t, repo, "other.txt", "o\n")
	commitAll(t, repo, "clean")
	gitT(t, repo, "checkout", "-q", "main")
	headBefore := gitT(t, repo, "rev-parse", "HEAD")
	statusBefore := gitT(t, repo, "status", "--porcelain")

	got, err := g.MergeConflicts(ctx, repo, "left", "right")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a b.txt", "c.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("conflicts = %q, want %q", got, want)
	}
	if got, err := g.MergeConflicts(ctx, repo, "left", "clean"); err != nil || got != nil {
		t.Errorf("clean merge = %v, %v", got, err)
	}
	if got, err := g.MergeConflicts(ctx, repo, "left", "left"); err != nil || got != nil {
		t.Errorf("self merge = %v, %v", got, err)
	}
	if _, err := g.MergeConflicts(ctx, repo, "left", "no-such-branch"); err == nil {
		t.Error("unknown revision should be an error, not a conflict")
	}
	if _, err := g.MergeConflicts(ctx, repo, "--bad", "left"); err == nil {
		t.Error("option-like revision should be rejected")
	}
	if gitT(t, repo, "rev-parse", "HEAD") != headBefore || gitT(t, repo, "status", "--porcelain") != statusBefore {
		t.Error("MergeConflicts touched the repository")
	}
}

func TestMergeConflictsUnrelatedHistories(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	gitT(t, repo, "checkout", "-q", "--orphan", "island")
	gitT(t, repo, "rm", "-rqf", ".")
	write(t, repo, "island.txt", "i\n")
	commitAll(t, repo, "island")
	if _, err := g.MergeConflicts(ctx, repo, "main", "island"); err == nil {
		t.Error("unrelated histories should be an error")
	}
}
