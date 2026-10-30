package gitcli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/vcs"
)

// diffFixture builds a repo with files on main, a worktree branched from it,
// then advances main (which must not show up in the session's diff).
func diffFixture(t *testing.T, g *Git) (repo, wt string) {
	t.Helper()
	repo = newRepo(t)
	var nums strings.Builder
	for i := 1; i <= 40; i++ {
		nums.WriteString("line " + string(rune('a'+i%26)) + "\n")
	}
	write(t, repo, "keep.txt", "one\ntwo\nthree\n")
	write(t, repo, "old name.txt", nums.String())
	write(t, repo, "gone.txt", "bye\n")
	write(t, repo, "img.bin", "\x00\x01\x02\x03binary")
	write(t, repo, ".gitignore", "*.log\n")
	commitAll(t, repo, "base files")
	wt = newWorktree(t, g, repo, "s1")
	// main moves on after the worktree was created.
	write(t, repo, "main-only.txt", "m\n")
	commitAll(t, repo, "main advances")
	return repo, wt
}

func TestDiffAndChangedFiles(t *testing.T) {
	g := needGit(t)
	repo, wt := diffFixture(t, g)

	// Session work: one committed change, then uncommitted ones.
	write(t, wt, "committed.txt", "c\n")
	commitAll(t, wt, "session commit")
	write(t, wt, "keep.txt", "one\nTWO\nthree\nfour\n")              // modified
	if err := os.Remove(filepath.Join(wt, "gone.txt")); err != nil { // deleted
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(wt, "old name.txt"), filepath.Join(wt, "new name.txt")); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(filepath.Join(wt, "new name.txt"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("appended\n")
	f.Close()
	write(t, wt, "img.bin", "\x00\x09\x09 changed binary\x00") // binary modified
	write(t, wt, "sub dir/untracked file.txt", "u1\nu2\n")     // untracked, spaces
	write(t, wt, "ignored.log", "noise\n")                     // ignored
	gitT(t, wt, "add", "committed.txt")                        // partly staged state must not matter
	before := indexState(t, wt)

	files, err := g.ChangedFiles(ctx, wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	want := []vcs.FileChange{
		{Path: "committed.txt", Status: vcs.Added, Added: 1},
		{Path: "gone.txt", Status: vcs.Deleted, Deleted: 1},
		{Path: "img.bin", Status: vcs.Modified, Binary: true},
		{Path: "keep.txt", Status: vcs.Modified, Added: 2, Deleted: 1},
		{Path: "new name.txt", OldPath: "old name.txt", Status: vcs.Renamed, Added: 1},
		{Path: "sub dir/untracked file.txt", Status: vcs.Added, Added: 2},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("ChangedFiles =\n%+v\nwant\n%+v", files, want)
	}

	diff, err := g.Diff(ctx, wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"diff --git a/keep.txt b/keep.txt",
		"+TWO",
		"rename from old name.txt",
		"rename to new name.txt",
		"deleted file mode",
		"+++ b/sub dir/untracked file.txt",
		"+u1",
		"Binary files a/img.bin and b/img.bin differ",
		"+++ b/committed.txt",
	} {
		if !strings.Contains(diff, s) {
			t.Errorf("diff missing %q\n%s", s, diff)
		}
	}
	for _, s := range []string{"main-only.txt", "ignored.log", "\x1b["} {
		if strings.Contains(diff, s) {
			t.Errorf("diff must not contain %q", s)
		}
	}
	if after := indexState(t, wt); after != before {
		t.Errorf("index/HEAD changed:\nbefore %s\nafter %s", before, after)
	}
	if st := gitT(t, wt, "status", "--porcelain"); !strings.Contains(st, "sub dir/") {
		t.Errorf("untracked file was staged in the real index:\n%s", st)
	}
	_ = repo
}

func TestDiffEmptyAndErrors(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "clean")
	d, err := g.Diff(ctx, wt, "main")
	if err != nil || d != "" {
		t.Errorf("clean diff = %q, %v", d, err)
	}
	files, err := g.ChangedFiles(ctx, wt, "main")
	if err != nil || len(files) != 0 {
		t.Errorf("clean files = %v, %v", files, err)
	}
	if _, err := g.Diff(ctx, wt, "no-such-branch"); err == nil {
		t.Error("expected error for unknown base")
	}
	if _, err := g.ChangedFiles(ctx, wt, "--output=x"); err == nil {
		t.Error("expected error for option-like base")
	}
}

func TestDiffIgnoresUserConfig(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "cfg")
	gitT(t, repo, "config", "diff.noprefix", "true")
	gitT(t, repo, "config", "diff.external", "false")
	gitT(t, repo, "config", "color.ui", "always")
	write(t, wt, "README.md", "hello\nworld\n")
	d, err := g.Diff(ctx, wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "--- a/README.md") || strings.Contains(d, "\x1b[") {
		t.Errorf("config leaked into diff:\n%s", d)
	}
}

func TestDiffUnicodePath(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "uni")
	write(t, wt, "café/naïve.txt", "x\n")
	d, err := g.Diff(ctx, wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "b/café/naïve.txt") {
		t.Errorf("path was quoted or mangled:\n%s", d)
	}
	files, err := g.ChangedFiles(ctx, wt, "main")
	if err != nil || len(files) != 1 || files[0].Path != "café/naïve.txt" {
		t.Errorf("files = %+v, %v", files, err)
	}
}

// An agent that is in the middle of a git command holds index.lock; reading
// and checkpointing must neither wait for it nor fail.
func TestNoRealIndexLocking(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "lock")
	write(t, wt, "a.txt", "a\n")
	lock := strings.TrimSpace(gitT(t, wt, "rev-parse", "--path-format=absolute", "--git-path", "index.lock"))
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(lock)
	if _, err := g.Diff(ctx, wt, "main"); err != nil {
		t.Errorf("Diff with index.lock held: %v", err)
	}
	if _, err := g.ChangedFiles(ctx, wt, "main"); err != nil {
		t.Errorf("ChangedFiles with index.lock held: %v", err)
	}
	if _, err := g.Checkpoint(ctx, wt, "refs/multiplex/checkpoints/lock/1", "m"); err != nil {
		t.Errorf("Checkpoint with index.lock held: %v", err)
	}
}

func TestParseChangesErrors(t *testing.T) {
	for _, in := range []string{":bad\x00x\x00", ":100644 100644 a b R100\x00only\x00", ":100644 100644 a b M\x00f\x00garbage\x00"} {
		if _, err := parseChanges([]byte(in)); err == nil {
			t.Errorf("parseChanges(%q) should fail", in)
		}
	}
	if got, err := parseChanges(nil); err != nil || got != nil {
		t.Errorf("empty = %v, %v", got, err)
	}
}
