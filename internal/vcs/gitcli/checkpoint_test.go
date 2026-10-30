package gitcli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCheckpoint(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	write(t, repo, ".gitignore", "*.log\n")
	write(t, repo, "staged-then-edited.txt", "v1\n")
	commitAll(t, repo, "setup")
	wt := newWorktree(t, g, repo, "cp")
	write(t, wt, "README.md", "modified\n")
	write(t, wt, "new dir/untracked file.txt", "u\n")
	write(t, wt, "ignored.log", "x\n")
	write(t, wt, "staged-then-edited.txt", "v2\n")
	gitT(t, wt, "add", "staged-then-edited.txt")
	write(t, wt, "staged-then-edited.txt", "v3\n")
	before := indexState(t, wt)
	statusBefore := gitT(t, wt, "status", "--porcelain")

	ref := "refs/multiplex/checkpoints/s1/1"
	commit, err := g.Checkpoint(ctx, wt, ref, "before rm -rf")
	if err != nil {
		t.Fatal(err)
	}
	if got := gitT(t, wt, "rev-parse", ref); got != commit {
		t.Errorf("ref = %s, want %s", got, commit)
	}
	if got := gitT(t, wt, "rev-parse", commit+"^"); got != gitT(t, wt, "rev-parse", "HEAD") {
		t.Errorf("parent = %s, want HEAD", got)
	}
	if got := gitT(t, wt, "log", "-1", "--format=%an <%ae>|%s", commit); got != "multiplex <multiplex@localhost>|before rm -rf" {
		t.Errorf("commit = %q", got)
	}
	files := strings.Fields(strings.ReplaceAll(gitT(t, wt, "ls-tree", "-r", "--name-only", commit), " ", "_"))
	sort.Strings(files)
	want := []string{".gitignore", "README.md", "new_dir/untracked_file.txt", "staged-then-edited.txt"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Errorf("tree = %v, want %v", files, want)
	}
	if got := gitT(t, wt, "show", commit+":staged-then-edited.txt"); got != "v3" {
		t.Errorf("checkpoint holds %q, want worktree content v3", got)
	}
	if after := indexState(t, wt); after != before {
		t.Errorf("index/HEAD changed:\nbefore %s\nafter %s", before, after)
	}
	if after := gitT(t, wt, "status", "--porcelain"); after != statusBefore {
		t.Errorf("status changed:\n%s\nvs\n%s", statusBefore, after)
	}
	if b, _ := os.ReadFile(filepath.Join(wt, "README.md")); string(b) != "modified\n" {
		t.Error("working file changed")
	}

	// A second checkpoint of the same state, and a branch that did not move.
	c2, err := g.Checkpoint(ctx, wt, "refs/multiplex/checkpoints/s1/2", "")
	if err != nil || c2 == "" {
		t.Fatalf("second checkpoint: %q %v", c2, err)
	}
	if got := gitT(t, wt, "log", "-1", "--format=%s", c2); got != "multiplex checkpoint" {
		t.Errorf("default message = %q", got)
	}
}

func TestCheckpointWithoutIdentity(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	gitT(t, repo, "config", "--unset", "user.name")
	gitT(t, repo, "config", "--unset", "user.email")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("EMAIL", "")
	write(t, repo, "x.txt", "x\n")
	if _, err := g.Checkpoint(ctx, repo, "refs/multiplex/checkpoints/id/1", "m"); err != nil {
		t.Fatalf("checkpoint without identity: %v", err)
	}
}

func TestCheckpointBadRef(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	for _, ref := range []string{"", "main", "--x", "heads/main"} {
		if _, err := g.Checkpoint(ctx, repo, ref, "m"); err == nil {
			t.Errorf("ref %q should be rejected", ref)
		}
	}
	if gitT(t, repo, "rev-parse", "--abbrev-ref", "HEAD") != "main" {
		t.Error("HEAD moved")
	}
}

func TestRestore(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	write(t, repo, ".gitignore", "*.log\n")
	write(t, repo, "tracked.txt", "t1\n")
	write(t, repo, "deleted me.txt", "d\n")
	write(t, repo, "dir/inner.txt", "i\n")
	commitAll(t, repo, "setup")
	wt := newWorktree(t, g, repo, "rs")

	// State to be captured: a tracked edit and an untracked file.
	write(t, wt, "tracked.txt", "t2 at checkpoint\n")
	write(t, wt, "untracked at cp/file.txt", "keep me\n")
	write(t, wt, "ignored.log", "ignored content\n")
	commit, err := g.Checkpoint(ctx, wt, "refs/multiplex/checkpoints/rs/1", "cp")
	if err != nil {
		t.Fatal(err)
	}
	gitT(t, wt, "add", "tracked.txt") // leave something staged
	before := indexState(t, wt)

	// The agent wrecks the worktree.
	write(t, wt, "tracked.txt", "wrecked\n")           // modify tracked
	write(t, wt, "agent-new.txt", "new\n")             // add untracked
	write(t, wt, "agent dir/deep/new.txt", "new\n")    // untracked in new dirs
	write(t, wt, "ignored.log", "changed ignored\n")   // ignored: must stay as is
	os.Remove(filepath.Join(wt, "deleted me.txt"))     // delete tracked
	os.RemoveAll(filepath.Join(wt, "untracked at cp")) // delete cp-untracked
	os.RemoveAll(filepath.Join(wt, "dir"))             // delete a tracked dir
	gitT(t, wt, "add", "agent-new.txt")                // index entry not in checkpoint
	before = indexState(t, wt)

	if err := g.Restore(ctx, wt, commit); err != nil {
		t.Fatal(err)
	}
	if after := indexState(t, wt); after != before {
		t.Errorf("index/HEAD changed by Restore:\nbefore %s\nafter %s", before, after)
	}

	// Worktree files (excluding ignored) must equal the checkpoint tree exactly.
	wantFiles := strings.Split(gitT(t, wt, "ls-tree", "-r", "--name-only", "-z", commit), "\x00")
	var want []string
	for _, f := range wantFiles {
		if f != "" {
			want = append(want, f)
		}
	}
	sort.Strings(want)
	var got []string
	filepath.WalkDir(wt, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == ".git" || d.Name() == "ignored.log" {
			return nil
		}
		rel, _ := filepath.Rel(wt, p)
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(got)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("files after restore:\n got %q\nwant %q", got, want)
	}
	for _, f := range want {
		disk, err := os.ReadFile(filepath.Join(wt, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		wantContent, _ := g.run(ctx, runOpts{dir: wt}, "show", commit+":"+f)
		if string(disk) != string(wantContent) {
			t.Errorf("%s = %q, want %q", f, disk, wantContent)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(wt, "ignored.log")); string(b) != "changed ignored\n" {
		t.Errorf("ignored file was touched: %q", b)
	}
	if _, err := os.Stat(filepath.Join(wt, "agent dir")); !os.IsNotExist(err) {
		t.Error("empty directories left behind")
	}
	// A fresh checkpoint now equals the restored one's tree.
	c2, err := g.Checkpoint(ctx, wt, "refs/multiplex/checkpoints/rs/2", "again")
	if err != nil {
		t.Fatal(err)
	}
	if gitT(t, wt, "rev-parse", c2+"^{tree}") != gitT(t, wt, "rev-parse", commit+"^{tree}") {
		t.Error("tree differs from checkpoint after restore")
	}
}

func TestRestoreFileDirectorySwap(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "swap")
	write(t, wt, "thing/file.txt", "inside\n")
	commit, err := g.Checkpoint(ctx, wt, "refs/multiplex/checkpoints/sw/1", "cp")
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(wt, "thing"))
	write(t, wt, "thing", "now a file\n")
	if err := g.Restore(ctx, wt, commit); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(wt, "thing", "file.txt")); err != nil || string(b) != "inside\n" {
		t.Errorf("thing/file.txt = %q, %v", b, err)
	}
}

func TestRestoreKeepsNestedRepo(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "nest")
	commit, err := g.Checkpoint(ctx, wt, "refs/multiplex/checkpoints/ne/1", "cp")
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(wt, "vendor-repo")
	os.MkdirAll(nested, 0o755)
	gitT(t, nested, "init", "-q")
	write(t, nested, "precious.txt", "p\n")
	if err := g.Restore(ctx, wt, commit); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(nested, "precious.txt")); err != nil {
		t.Errorf("nested repository content was deleted: %v", err)
	}
}

func TestRestoreBadCommit(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	write(t, repo, "keep.txt", "k\n")
	for _, c := range []string{"", "nope", "-f"} {
		if err := g.Restore(ctx, repo, c); err == nil {
			t.Errorf("Restore(%q) should fail", c)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "keep.txt")); err != nil {
		t.Error("a failed restore deleted files")
	}
}
