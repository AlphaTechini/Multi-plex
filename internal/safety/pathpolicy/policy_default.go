package pathpolicy

import (
	"os"
	"path"
	"runtime"
)

// Dirs are the per-user locations the default lists are built from.
type Dirs struct {
	Home         string // user home
	Temp         string // os.TempDir
	Cache        string // os.UserCacheDir
	Config       string // os.UserConfigDir
	AppData      string // %APPDATA% (Windows only)
	LocalAppData string // %LOCALAPPDATA% (Windows only)
}

// HostDirs reads Dirs from the running system.
func HostDirs() Dirs {
	var d Dirs
	d.Home, _ = os.UserHomeDir()
	d.Temp = os.TempDir()
	d.Cache, _ = os.UserCacheDir()
	d.Config, _ = os.UserConfigDir()
	if runtime.GOOS == "windows" {
		d.AppData = os.Getenv("APPDATA")
		d.LocalAppData = os.Getenv("LOCALAPPDATA")
	}
	return d
}

// DefaultPolicy returns the default Layer 2 and 3 policy for a worktree on
// this machine. Callers append workspace protected paths and user entries;
// they must never remove defaults (docs/04: never weaken a default).
func DefaultPolicy(worktreeRoot string) Policy {
	return DefaultPolicyFor(runtime.GOOS, worktreeRoot, HostDirs())
}

// DefaultPolicyFor builds the default policy for any GOOS from explicit
// directories, so every platform's lists are testable on one machine.
func DefaultPolicyFor(goos, worktreeRoot string, d Dirs) Policy {
	return Policy{
		WorktreeRoot:      worktreeRoot,
		WritableExtra:     DefaultWritableExtra(goos, d),
		DenyRead:          DefaultDenyRead(goos, d),
		ProtectedSettings: DefaultProtectedSettings(goos, d),
		Home:              d.Home,
		goos:              goos,
		noResolve:         goos != runtime.GOOS,
	}
}

// DefaultWritableExtra lists the small allow-list of writable places outside
// the worktree: temp and tool caches (docs/04 Layer 2).
func DefaultWritableExtra(goos string, d Dirs) []string {
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}
	add(d.Temp)
	if goos != "windows" {
		add("/tmp")
		add("/var/tmp")
	}
	add(d.Cache)
	if d.Home != "" {
		add(d.Home + "/.npm")
		add(d.Home + "/.cache")
		add(d.Home + "/go/pkg")
		add(d.Home + "/.cargo/registry")
		add(d.Home + "/.cargo/git")
		add(d.Home + "/.local/share/pnpm")
	}
	if goos == "windows" && d.LocalAppData != "" {
		add(d.LocalAppData + `\npm-cache`)
		add(d.LocalAppData + `\pnpm`)
		add(d.LocalAppData + `\go-build`)
	}
	return out
}

// DefaultProtectedSettings lists files that can disable or reroute the safety
// layer. Relative entries match at any depth.
func DefaultProtectedSettings(goos string, d Dirs) []string {
	out := []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".multiplex/safety-rules.json",
		".git/hooks",
		".git/config",
	}
	if d.Home != "" {
		out = append(out, d.Home+"/.claude/settings.json", d.Home+"/.claude.json")
	}
	if d.Config != "" {
		out = append(out, path.Join(slash(d.Config), "multiplex", "safety-rules.json"))
	}
	switch goos {
	case "windows":
		out = append(out, `C:\ProgramData\ClaudeCode\managed-settings.json`)
	case "darwin":
		out = append(out, "/Library/Application Support/ClaudeCode/managed-settings.json")
	default:
		out = append(out, "/etc/claude-code/managed-settings.json")
	}
	return out
}

func slash(p string) string {
	b := []byte(p)
	for i, c := range b {
		if c == '\\' {
			b[i] = '/'
		}
	}
	return string(b)
}

// DefaultDenyRead lists credential stores that agent file tools must not read
// (docs/04 Layer 3): cloud and VCS credentials, SSH and GPG keys, registry
// tokens, browser profiles and password-manager data.
func DefaultDenyRead(goos string, d Dirs) []string {
	var out []string
	h := d.Home
	if h != "" {
		for _, rel := range []string{
			".aws", ".config/gcloud", ".ssh", ".kube", ".docker/config.json", ".azure",
			".gnupg", ".netrc", "_netrc", ".git-credentials", ".config/git/credentials",
			".npmrc", ".pypirc", ".config/gh", ".oci", ".terraform.d/credentials.tfrc.json",
			".cargo/credentials", ".cargo/credentials.toml", ".pgpass", ".vault-token",
			".config/doctl", ".config/op", ".password-store", ".m2/settings-security.xml",
			".claude/.credentials.json",
		} {
			out = append(out, h+"/"+rel)
		}
		switch goos {
		case "darwin":
			for _, rel := range []string{
				"Library/Application Support/Google/Chrome",
				"Library/Application Support/Microsoft Edge",
				"Library/Application Support/BraveSoftware",
				"Library/Application Support/Firefox",
				"Library/Application Support/1Password",
				"Library/Group Containers/2BUA8C4S2C.com.1password",
				"Library/Application Support/Bitwarden",
				"Library/Application Support/KeePassXC",
				"Library/Keychains", "Library/Cookies", "Library/Safari",
			} {
				out = append(out, h+"/"+rel)
			}
		case "windows":
			// handled below with APPDATA/LOCALAPPDATA
		default:
			for _, rel := range []string{
				".config/google-chrome", ".config/chromium", ".config/microsoft-edge",
				".config/BraveSoftware", ".mozilla", ".config/1Password", ".config/Bitwarden",
				".config/KeePass", ".config/keepassxc", ".keepass", ".local/share/keyrings",
				"snap/chromium/common/chromium", "snap/firefox/common/.mozilla",
			} {
				out = append(out, h+"/"+rel)
			}
		}
	}
	if goos == "windows" {
		local, roam := d.LocalAppData, d.AppData
		if local == "" && h != "" {
			local = h + `\AppData\Local`
		}
		if roam == "" && h != "" {
			roam = h + `\AppData\Roaming`
		}
		for _, rel := range []string{
			`Google\Chrome`, `Microsoft\Edge`, `BraveSoftware`, `1Password`, `Opera Software`,
			`Microsoft\Credentials`, `Microsoft\Vault`,
		} {
			if local != "" {
				out = append(out, local+`\`+rel)
			}
		}
		for _, rel := range []string{
			`Mozilla\Firefox`, `Bitwarden`, `KeePass`, `gcloud`, `GitHub CLI`, `terraform.d\credentials.tfrc.json`,
			`postgresql\pgpass.conf`, `Microsoft\Credentials`, `Microsoft\Protect`, `Microsoft\Vault`,
		} {
			if roam != "" {
				out = append(out, roam+`\`+rel)
			}
		}
	}
	return out
}
