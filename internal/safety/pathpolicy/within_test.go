package pathpolicy

import "testing"

func TestWithinOS(t *testing.T) {
	cases := []struct {
		name, goos, root, path string
		want                   bool
	}{
		{"posix equal", "linux", "/a/b", "/a/b", true},
		{"posix child", "linux", "/a/b", "/a/b/c/d", true},
		{"posix sibling prefix", "linux", "/a/b", "/a/bc", false},
		{"posix parent", "linux", "/a/b", "/a", false},
		{"posix dotdot escape", "linux", "/a/b", "/a/b/../c", false},
		{"posix dotdot inside", "linux", "/a/b", "/a/b/c/../d", true},
		{"posix relative inside", "linux", "/a/b", "c/d", true},
		{"posix relative escape", "linux", "/a/b", "../c", false},
		{"posix case sensitive", "linux", "/a/b", "/A/B/c", false},
		{"posix root", "linux", "/", "/etc/passwd", true},
		{"empty root", "linux", "", "/etc", false},
		{"win drive child", "windows", `C:\a`, `C:\a\b`, true},
		{"win prefix sibling", "windows", `C:\a`, `C:\ab`, false},
		{"win prefix sibling slash", "windows", `C:\a`, `C:/ab/x`, false},
		{"win mixed slashes", "windows", `C:\a\b`, `c:/A/B/c`, true},
		{"win other drive", "windows", `C:\a`, `D:\a\b`, false},
		{"win drive root", "windows", `C:\`, `C:\Windows`, true},
		{"win drive root other", "windows", `C:\`, `D:\x`, false},
		{"win case", "windows", `C:\Work\Proj`, `c:\work\proj\SRC\main.go`, true},
		{"win dotdot", "windows", `C:\a\b`, `C:\a\b\..\c`, false},
		{"win relative", "windows", `C:\a\b`, `src\x.go`, true},
		{"win relative escape", "windows", `C:\a\b`, `..\..\Windows`, false},
		{"win unc child", "windows", `\\srv\share\a`, `\\srv\share\a\b`, true},
		{"win unc other share", "windows", `\\srv\share\a`, `\\srv\share2\a\b`, false},
		{"win unc case", "windows", `\\SRV\Share`, `\\srv\share\x`, true},
		{"win long path prefix", "windows", `C:\a`, `\\?\C:\a\b`, true},
		{"win long unc prefix", "windows", `\\srv\share`, `\\?\UNC\srv\share\x`, true},
		{"win trailing dots ignored", "windows", `C:\a`, `C:\a.\..\b`, false},
		{"win rooted no drive", "windows", `C:\a`, `\a\b`, true},
		{"darwin case-insens", "darwin", "/Users/Me/proj", "/users/me/PROJ/x", true},
		{"darwin sibling", "darwin", "/Users/Me/proj", "/users/me/proj2", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WithinOS(c.goos, c.root, c.path); got != c.want {
				t.Fatalf("WithinOS(%q,%q,%q) = %v, want %v", c.goos, c.root, c.path, got, c.want)
			}
		})
	}
}

func TestWithinHost(t *testing.T) {
	if !Within(t.TempDir(), "x") {
		t.Fatal("relative path should be inside root")
	}
}

func TestGuessOS(t *testing.T) {
	for p, want := range map[string]string{`C:\x`: "windows", `\\srv\x`: "windows", `d:/x`: "windows"} {
		if got := GuessOS(p); got != want {
			t.Errorf("GuessOS(%q)=%q want %q", p, got, want)
		}
	}
	if GuessOS("/x") == "windows" {
		t.Error("posix path guessed as windows")
	}
}

func TestCleanOS(t *testing.T) {
	cases := []struct{ goos, in, want string }{
		{"windows", `C:/a/./b/../c`, `C:\a\c`},
		{"windows", `c:\`, `c:\`},
		{"windows", `C:\a\b\..\..\..`, `C:\`},
		{"windows", `\\srv\share\a\..`, `\\srv\share\`},
		{"windows", `\\?\C:\x\y`, `C:\x\y`},
		{"linux", `/a//b/../c/`, "/a/c"},
	}
	for _, c := range cases {
		if got := CleanOS(c.goos, c.in); got != c.want {
			t.Errorf("CleanOS(%q,%q)=%q want %q", c.goos, c.in, got, c.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	cases := []struct {
		goos, in, want string
		ok             bool
	}{
		{"linux", "~", "/h", true},
		{"linux", "~/x", "/h/x", true},
		{"linux", "~root/x", "~root/x", false},
		{"linux", "a/~", "a/~", true},
		{"windows", `~\x`, `/h/x`, true},
		{"linux", `~\x`, `~\x`, false},
	}
	for _, c := range cases {
		got, ok := ExpandHome(c.goos, "/h", c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ExpandHome(%q,%q)=(%q,%v) want (%q,%v)", c.goos, c.in, got, ok, c.want, c.ok)
		}
	}
}
