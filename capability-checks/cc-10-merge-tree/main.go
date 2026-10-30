// Command cc-10-merge-tree checks that `git merge-tree --write-tree` predicts
// merge conflicts correctly without touching the working tree, index or refs.
//
// It creates a throwaway repository in the system temp directory, builds a
// base commit and three branches, runs merge-tree on a clean pair and on a
// conflicting pair, and prints PASS or FAIL. Standard library only, no shell,
// so it runs the same on Windows, macOS and Linux.
//
//	go run ./capability-checks/cc-10-merge-tree
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// gitEnv isolates the test repository from the user's global and system git
// config (signing, hooks, templates), so the check never prompts and is
// deterministic. merge-tree itself does not depend on that config.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_NAME=cc10", "GIT_AUTHOR_EMAIL=cc10@example.invalid",
		"GIT_COMMITTER_NAME=cc10", "GIT_COMMITTER_EMAIL=cc10@example.invalid",
		"GIT_TERMINAL_PROMPT=0",
	)
}

// run runs git in dir and returns stdout, the exit code and an error for
// failures other than a non-zero exit.
func run(dir string, args ...string) (string, int, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.String(), exitErr.ExitCode(), nil
	}
	if err != nil {
		return "", -1, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), 0, nil
}

// must runs git and fails on any non-zero exit.
func must(dir string, args ...string) string {
	out, code, err := run(dir, args...)
	if err != nil {
		fatal(err)
	}
	if code != 0 {
		fatal(fmt.Errorf("git %s exited %d", strings.Join(args, " "), code))
	}
	return out
}

func fatal(err error) {
	fmt.Println("FAIL:", err)
	os.Exit(1)
}

func write(root, rel, content string) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		fatal(err)
	}
}

func commitAll(dir, msg string) {
	must(dir, "add", "-A")
	must(dir, "commit", "-q", "-m", msg)
}

// mergeResult is the parsed output of `git merge-tree --write-tree --name-only -z`.
type mergeResult struct {
	Tree       string
	Conflicted []string // workspace-relative paths, forward slashes, sorted, unique
	Clean      bool     // exit status 0
	Elapsed    time.Duration
}

// predictMerge runs merge-tree for two commits. With -z the output is:
//
//	<tree-oid> NUL <path> NUL <path> NUL ... NUL <informational messages>
//
// With --name-only each conflicted path appears once. --no-messages drops the
// informational section, so everything after the tree OID is a path. Exit
// status is 0 for a clean merge, 1 for conflicts, anything else is an error.
// Requires git 2.38 or later.
func predictMerge(dir, ours, theirs string) (mergeResult, error) {
	start := time.Now()
	out, code, err := run(dir, "merge-tree", "--write-tree", "--name-only", "--no-messages", "-z", ours, theirs)
	elapsed := time.Since(start)
	if err != nil {
		return mergeResult{}, err
	}
	if code != 0 && code != 1 {
		return mergeResult{}, fmt.Errorf("merge-tree exited %d (git too old? needs 2.38+)", code)
	}
	fields := strings.Split(strings.TrimRight(out, "\x00"), "\x00")
	res := mergeResult{Tree: strings.TrimSpace(fields[0]), Clean: code == 0, Elapsed: elapsed}
	for _, f := range fields[1:] {
		if f != "" && !slices.Contains(res.Conflicted, f) {
			res.Conflicted = append(res.Conflicted, f)
		}
	}
	slices.Sort(res.Conflicted)
	return res, nil
}

// snapshot captures everything merge-tree must not change: HEAD, all refs,
// the index file, status output and the bytes of every working-tree file.
func snapshot(dir string) string {
	h := sha256.New()
	fmt.Fprintln(h, must(dir, "rev-parse", "HEAD"))
	fmt.Fprintln(h, must(dir, "symbolic-ref", "HEAD"))
	fmt.Fprintln(h, must(dir, "for-each-ref", "--format=%(refname) %(objectname)"))
	fmt.Fprintln(h, must(dir, "status", "--porcelain=v1", "-z", "--untracked-files=all"))
	if idx, err := os.ReadFile(filepath.Join(dir, ".git", "index")); err == nil {
		h.Write(idx)
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %x\n", filepath.ToSlash(rel), sha256.Sum256(b))
		return nil
	})
	if err != nil {
		fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	verOut, _, err := run(".", "--version")
	if err != nil {
		fatal(err)
	}
	fmt.Println(strings.TrimSpace(verOut))

	dir, err := os.MkdirTemp("", "cc10-merge-tree-")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(dir)

	must(dir, "init", "-q", "-b", "main")
	must(dir, "config", "core.autocrlf", "false")

	// Base commit.
	write(dir, "shared.txt", "line 1\nline 2\nline 3\nline 4\nline 5\n")
	write(dir, "deleted-on-b.txt", "keep me\n")
	write(dir, "dir with space/notes.md", "alpha\nbeta\ngamma\n")
	write(dir, "untouched.go", "package x\n")
	commitAll(dir, "base")

	// Branch a: edits that conflict with b on some files, and a disjoint edit.
	must(dir, "switch", "-q", "-c", "a")
	write(dir, "shared.txt", "line 1\nline 2 from A\nline 3\nline 4\nline 5\n")
	write(dir, "deleted-on-b.txt", "keep me, edited on A\n")
	write(dir, "dir with space/notes.md", "alpha\nbeta from A\ngamma\n")
	write(dir, "added-both.txt", "added on A\n")
	write(dir, "only-a.txt", "A only\n")
	commitAll(dir, "a")

	// Branch b: conflicting edits (content, modify/delete, add/add, path with a space).
	must(dir, "switch", "-q", "-c", "b", "main")
	write(dir, "shared.txt", "line 1\nline 2 from B\nline 3\nline 4\nline 5\n")
	if err := os.Remove(filepath.Join(dir, "deleted-on-b.txt")); err != nil {
		fatal(err)
	}
	write(dir, "dir with space/notes.md", "alpha\nbeta from B\ngamma\n")
	write(dir, "added-both.txt", "added on B\n")
	commitAll(dir, "b")

	// Branch c: does not overlap with a.
	must(dir, "switch", "-q", "-c", "c", "main")
	write(dir, "shared.txt", "line 1\nline 2\nline 3\nline 4\nline 5 from C\n")
	write(dir, "only-c.txt", "C only\n")
	commitAll(dir, "c")

	// Leave the working tree on main with an uncommitted change and an
	// untracked file, the state an agent's worktree is usually in.
	must(dir, "switch", "-q", "main")
	write(dir, "untouched.go", "package x // dirty\n")
	write(dir, "untracked.tmp", "scratch\n")

	before := snapshot(dir)
	failed := false
	check := func(ok bool, format string, args ...any) {
		status := "ok  "
		if !ok {
			status = "FAIL"
			failed = true
		}
		fmt.Printf("%s %s\n", status, fmt.Sprintf(format, args...))
	}

	clean, err := predictMerge(dir, "a", "c")
	if err != nil {
		fatal(err)
	}
	treeType, _, _ := run(dir, "cat-file", "-t", clean.Tree)
	check(clean.Clean && len(clean.Conflicted) == 0 && strings.TrimSpace(treeType) == "tree",
		"a+c: clean=%v conflicts=%q tree=%s (%s)", clean.Clean, clean.Conflicted, short(clean.Tree), clean.Elapsed.Round(time.Millisecond))

	conf, err := predictMerge(dir, "a", "b")
	if err != nil {
		fatal(err)
	}
	want := []string{"added-both.txt", "deleted-on-b.txt", "dir with space/notes.md", "shared.txt"}
	check(!conf.Clean && slices.Equal(conf.Conflicted, want),
		"a+b: clean=%v conflicts=%q want %q (%s)", conf.Clean, conf.Conflicted, want, conf.Elapsed.Round(time.Millisecond))

	after := snapshot(dir)
	check(before == after, "working tree, index, HEAD and refs unchanged")

	if failed {
		fmt.Println("FAIL")
		os.Exit(1)
	}
	fmt.Println("PASS")
}

func short(oid string) string {
	if len(oid) > 12 {
		return oid[:12]
	}
	return oid
}
