package gitcli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	write(t, repo, "dir/with space.txt", "committed\n")
	commitAll(t, repo, "add")
	wt := newWorktree(t, g, repo, "rf")
	write(t, wt, "dir/with space.txt", "edited\n")

	tests := []struct {
		name, rev, path string
		want            string
		wantErr         bool
	}{
		{"disk", "", "dir/with space.txt", "edited\n", false},
		{"disk unclean path", "", "dir/../dir/with space.txt", "edited\n", false},
		{"HEAD", "HEAD", "dir/with space.txt", "committed\n", false},
		{"branch", "main", "README.md", "hello\n", false},
		{"missing on disk", "", "nope.txt", "", true},
		{"missing in rev", "HEAD", "nope.txt", "", true},
		{"dotdot", "", "../x", "", true},
		{"dotdot nested", "", "dir/../../x", "", true},
		{"dotdot rev", "HEAD", "../x", "", true},
		{"absolute", "", filepath.ToSlash(filepath.Join(wt, "README.md")), "", true},
		{"root slash", "", "/etc/passwd", "", true},
		{"dot", "", ".", "", true},
		{"empty", "", "", "", true},
		{"option rev", "--help", "README.md", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.ReadFile(ctx, wt, tt.rev, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadFileSymlinkEscape(t *testing.T) {
	g := needGit(t)
	repo := newRepo(t)
	wt := newWorktree(t, g, repo, "sl")
	outside := filepath.Join(realTemp(t), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(wt, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if b, err := g.ReadFile(ctx, wt, "", "link.txt"); err == nil {
		t.Errorf("symlink escape was readable: %q", b)
	}
	if err := os.Symlink("README.md", filepath.Join(wt, "inner.txt")); err != nil {
		t.Skip(err)
	}
	if b, err := g.ReadFile(ctx, wt, "", "inner.txt"); err != nil || string(b) != "hello\n" {
		t.Errorf("inner symlink = %q, %v", b, err)
	}
}
