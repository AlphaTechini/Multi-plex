package pathpolicy

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func winPolicy() Policy {
	d := Dirs{Home: `C:\Users\me`, Temp: `C:\Users\me\AppData\Local\Temp`, Cache: `C:\Users\me\AppData\Local`,
		Config: `C:\Users\me\AppData\Roaming`, AppData: `C:\Users\me\AppData\Roaming`, LocalAppData: `C:\Users\me\AppData\Local`}
	return DefaultPolicyFor("windows", `C:\work\proj`, d)
}

func posixPolicy(goos string) Policy {
	d := Dirs{Home: "/home/me", Temp: "/tmp", Cache: "/home/me/.cache", Config: "/home/me/.config"}
	if goos == "darwin" {
		d = Dirs{Home: "/Users/me", Temp: "/var/folders/xx/T", Cache: "/Users/me/Library/Caches", Config: "/Users/me/Library/Application Support"}
		return DefaultPolicyFor(goos, "/Users/me/proj", d)
	}
	return DefaultPolicyFor(goos, "/home/me/proj", d)
}

type wcase struct {
	name, path string
	allowed    bool
	rule       string
}

func runWrite(t *testing.T, p Policy, cases []wcase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := p.CheckWrite(c.path)
			if v.Allowed != c.allowed || (!c.allowed && v.Rule != c.rule) {
				t.Fatalf("CheckWrite(%q) = %+v, want allowed=%v rule=%q", c.path, v, c.allowed, c.rule)
			}
		})
	}
}

func TestCheckWriteWindows(t *testing.T) {
	p := winPolicy()
	runWrite(t, p, []wcase{
		{"relative inside", `src\main.go`, true, ""},
		{"forward slash relative", `src/main.go`, true, ""},
		{"absolute inside", `C:\work\proj\a.txt`, true, ""},
		{"absolute inside other case", `c:\WORK\Proj\a.txt`, true, ""},
		{"sibling prefix", `C:\work\proj2\a.txt`, false, RuleOutsideWorktree},
		{"dotdot escape", `..\other\a.txt`, false, RuleOutsideWorktree},
		{"other drive", `D:\work\proj\a.txt`, false, RuleOutsideWorktree},
		{"system dir", `C:\Windows\System32\drivers\etc\hosts`, false, RuleOutsideWorktree},
		{"unc", `\\srv\share\x`, false, RuleOutsideWorktree},
		{"temp allowed", `C:\Users\me\AppData\Local\Temp\x.tmp`, true, ""},
		{"home file", `~\notes.txt`, false, RuleOutsideWorktree},
		{"home ssh", `~\.ssh\config`, false, RuleDenyRead},
		{"claude settings in worktree", `.claude\settings.json`, false, RuleProtectedConfig},
		{"claude settings local", `C:\work\proj\sub\.claude\settings.local.json`, false, RuleProtectedConfig},
		{"user claude settings", `C:\Users\me\.claude\settings.json`, false, RuleProtectedConfig},
		{"user claude json", `~/.claude.json`, false, RuleProtectedConfig},
		{"safety rules", `.multiplex\safety-rules.json`, false, RuleProtectedConfig},
		{"multiplex other file ok", `.multiplex\specs\a.md`, true, ""},
		{"git hooks", `.git\hooks\pre-commit`, false, RuleProtectedConfig},
		{"chrome profile", `C:\Users\me\AppData\Local\Google\Chrome\User Data\Default\Login Data`, false, RuleDenyRead},
		{"unresolvable tilde user", `~bob\x`, false, RuleUnresolvable},
		{"empty", ``, false, RuleUnresolvable},
		{"trailing dot trick", `C:\work\proj\..\proj2.\x`, false, RuleOutsideWorktree},
		{"settings trailing dot", `.claude\settings.json.`, false, RuleProtectedConfig},
		{"long path prefix inside", `\\?\C:\work\proj\a.txt`, true, ""},
	})
}

func TestCheckWritePosixAndDarwin(t *testing.T) {
	runWrite(t, posixPolicy("linux"), []wcase{
		{"relative", "a/b.txt", true, ""},
		{"inside abs", "/home/me/proj/a", true, ""},
		{"sibling prefix", "/home/me/proj2/a", false, RuleOutsideWorktree},
		{"case differs is outside on linux", "/home/me/PROJ/a", false, RuleOutsideWorktree},
		{"etc", "/etc/passwd", false, RuleOutsideWorktree},
		{"tmp", "/tmp/x", true, ""},
		{"tmp dotdot", "/tmp/../etc/x", false, RuleOutsideWorktree},
		{"cache", "~/.cache/go-build/x", true, ""},
		{"bashrc", "~/.bashrc", false, RuleOutsideWorktree},
		{"aws", "~/.aws/credentials", false, RuleDenyRead},
		{"claude json", "~/.claude.json", false, RuleProtectedConfig},
		{"user claude settings", "/home/me/.claude/settings.json", false, RuleProtectedConfig},
		{"dotdot in middle", "sub/../../x", false, RuleOutsideWorktree},
	})
	runWrite(t, posixPolicy("darwin"), []wcase{
		{"case insensitive inside", "/users/ME/PROJ/a.txt", true, ""},
		{"case insensitive settings", "/Users/me/proj/.CLAUDE/Settings.json", false, RuleProtectedConfig},
		{"keychains", "~/Library/Keychains/login.keychain-db", false, RuleDenyRead},
		{"chrome", "/users/me/library/application support/google/chrome/Default/Cookies", false, RuleDenyRead},
	})
}

func TestCheckReadDeny(t *testing.T) {
	p := winPolicy()
	cases := []struct {
		path    string
		allowed bool
	}{
		{`C:\work\proj\main.go`, true},
		{`C:\Windows\notepad.exe`, true},
		{`~\.aws\credentials`, false},
		{`C:\Users\me\.AWS\config`, false},
		{`C:\Users\me\.ssh\id_ed25519`, false},
		{`C:\Users\me\AppData\Roaming\gcloud\credentials.db`, false},
		{`C:\Users\me\AppData\Roaming\Mozilla\Firefox\Profiles\x\logins.json`, false},
		{`C:\Users\me\AppData\Roaming\Bitwarden\data.json`, false},
		{`C:\Users\me\.docker\config.json`, false},
		{`C:\Users\me\.docker\other.json`, true},
		{`C:\Users\me\.npmrc`, false},
		{`C:\Users\me\.sshfoo\x`, true},
		{`C:\Users\me\AppData\Roaming\Microsoft\Protect\S-1\key`, false},
	}
	for _, c := range cases {
		if v := p.CheckRead(c.path); v.Allowed != c.allowed {
			t.Errorf("CheckRead(%q) = %+v, want allowed=%v", c.path, v, c.allowed)
		}
	}
	lp := posixPolicy("linux")
	for path, allowed := range map[string]bool{
		"~/.config/gcloud/application_default_credentials.json": false,
		"~/.kube/config":                  false,
		"~/.config/google-chrome/Default": false,
		"~/.gnupg/pubring.kbx":            false,
		"~/.netrc":                        false,
		"~/.git-credentials":              false,
		"~/.pypirc":                       false,
		"~/.azure/accessTokens.json":      false,
		"~/.claude/.credentials.json":     false,
		"~/.config/Bitwarden/data.json":   false,
		"~/.config/1Password/x":           false,
		"~/.bashrc":                       true,
		"/usr/include/stdio.h":            true,
	} {
		if v := lp.CheckRead(path); v.Allowed != allowed {
			t.Errorf("CheckRead(%q) = %+v, want allowed=%v", path, v, allowed)
		}
	}
}

func TestProtectedWorkspacePaths(t *testing.T) {
	p := posixPolicy("linux")
	p.Protected = []string{".env", ".env.*", "data/*.db", "secrets", "*.pem", "/home/me/other-important"}
	runWrite(t, p, []wcase{
		{"env", ".env", false, RuleProtectedPath},
		{"env local", "sub/.env.local", false, RuleProtectedPath},
		{"db glob", "data/app.db", false, RuleProtectedPath},
		{"db elsewhere", "other/app.db", true, ""},
		{"dir", "secrets/key.txt", false, RuleProtectedPath},
		{"pem anywhere", "a/b/c.pem", false, RuleProtectedPath},
		{"abs entry outside", "/home/me/other-important/x", false, RuleProtectedPath},
		{"normal", "main.go", true, ""},
		{"envrc not matched", ".envrc", true, ""},
	})
}

func TestSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs privileges on Windows")
	}
	base := t.TempDir()
	root := filepath.Join(base, "wt")
	outside := filepath.Join(base, "outside")
	secret := filepath.Join(base, "home", ".ssh")
	for _, d := range []string{root, outside, secret} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustLink := func(target, link string) {
		t.Helper()
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	mustLink(outside, filepath.Join(root, "escape"))
	mustLink(secret, filepath.Join(root, "keys"))
	mustLink(filepath.Join(outside, "newfile"), filepath.Join(root, "dangling"))
	mustLink(filepath.Join(root, "sub"), filepath.Join(root, "loop1"))
	mustLink(filepath.Join(root, "loop1"), filepath.Join(root, "loop2"))
	mustLink(filepath.Join(root, "loop2"), filepath.Join(root, "sub"))
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustLink(filepath.Join(root, "real"), filepath.Join(root, "inner"))

	p := Policy{WorktreeRoot: root, DenyRead: []string{filepath.Join(base, "home", ".ssh")},
		ProtectedSettings: []string{".claude/settings.json"}, Home: filepath.Join(base, "home")}
	runWrite(t, p, []wcase{
		{"plain", "a.txt", true, ""},
		{"via symlink escape", "escape/x.txt", false, RuleOutsideWorktree},
		{"via dangling symlink", "dangling", false, RuleOutsideWorktree},
		{"symlink to deny dir", "keys/id_rsa", false, RuleDenyRead},
		{"dotdot after symlink", "escape/../x.txt", false, RuleOutsideWorktree},
		{"symlink loop", "loop1/x", false, RuleUnresolvable},
		{"internal symlink fine", "inner/x.txt", true, ""},
		{"abs escape", filepath.Join(root, "escape", "y"), false, RuleOutsideWorktree},
	})
	if v := p.CheckRead("keys/id_rsa"); v.Allowed || v.Rule != RuleDenyRead {
		t.Fatalf("read through symlink to deny dir allowed: %+v", v)
	}
	// A symlink to a protected settings file is caught by its target.
	if err := os.MkdirAll(filepath.Join(outside, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustLink(filepath.Join(outside, ".claude", "settings.json"), filepath.Join(root, "innocent.json"))
	if v := p.CheckWrite("innocent.json"); v.Allowed || v.Rule != RuleProtectedConfig {
		t.Fatalf("symlink to settings allowed: %+v", v)
	}
}

func TestRootBehindSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	real := filepath.Join(base, "real")
	link := filepath.Join(base, "link")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	// The worktree is configured through the link; the agent uses the real path.
	p := Policy{WorktreeRoot: link}
	if v := p.CheckWrite(filepath.Join(real, "x")); !v.Allowed {
		t.Fatalf("real path under symlinked root rejected: %+v", v)
	}
	if v := p.CheckWrite(filepath.Join(link, "x")); !v.Allowed {
		t.Fatalf("link path rejected: %+v", v)
	}
}

func TestDisplay(t *testing.T) {
	p := winPolicy()
	cases := map[string]string{
		`C:\work\proj\src\a.go`: "src/a.go",
		`src\a.go`:              "src/a.go",
		`C:\work\proj`:          ".",
		`C:\Windows\x`:          "C:/Windows/x",
	}
	for in, want := range cases {
		if got := p.Display(in); got != want {
			t.Errorf("Display(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDefaultPolicyHost(t *testing.T) {
	p := DefaultPolicy(t.TempDir())
	if len(p.DenyRead) == 0 || len(p.ProtectedSettings) == 0 {
		t.Fatal("default policy is empty")
	}
	if v := p.CheckWrite("x.txt"); !v.Allowed {
		t.Fatalf("write inside worktree denied: %+v", v)
	}
}
