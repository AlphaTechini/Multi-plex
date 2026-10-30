package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealGitDiff runs the parser against genuine git output and checks that
// expanding every gap reproduces the new file exactly.
func TestRealGitDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	oldC := fileContent(100)
	newC := append([]string(nil), oldC...)
	newC[4] = "five changed"
	newC[49] = "fifty changed"
	newC = append(newC[:70], append([]string{"inserted a", "inserted b"}, newC[70:]...)...)
	newC = newC[:len(newC)-3] // drop the last 3 lines
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte(strings.Join(oldC, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "init")
	if err := os.WriteFile(path, []byte(strings.Join(newC, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := run("diff", "--no-color", "--histogram")

	files := mustParse(t, raw)
	if len(files) != 1 {
		t.Fatalf("files = %d", len(files))
	}
	f := &files[0]
	if len(f.Hunks) < 3 {
		t.Fatalf("hunks = %d\n%s", len(f.Hunks), raw)
	}
	ApplyWordDiff(f)
	// Expand from the back so earlier gap indices stay valid.
	for g := len(f.Hunks); g >= 0; g-- {
		ExpandGap(f, g, newC)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("hunks after expansion = %d", len(f.Hunks))
	}
	var got []string
	for _, l := range f.Hunks[0].Lines {
		if l.Kind != Del {
			got = append(got, l.Text)
		}
	}
	if strings.Join(got, "\n") != strings.Join(newC, "\n") {
		t.Errorf("expanded new side differs from the file (%d vs %d lines)", len(got), len(newC))
	}
	if h := f.Hunks[0]; h.NewLines != len(newC) || h.OldLines != len(oldC) || h.NewStart != 1 || h.OldStart != 1 {
		t.Errorf("hunk = %+v", h)
	}
}
