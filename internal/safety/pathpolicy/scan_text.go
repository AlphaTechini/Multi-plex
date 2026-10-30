package pathpolicy

import "strings"

// WritableDirs returns the worktree and the writable allow-list as absolute
// cleaned paths ("~" expanded).
func (p Policy) WritableDirs() []string {
	var out []string
	for _, e := range append([]string{p.WorktreeRoot}, p.WritableExtra...) {
		if abs, ok := p.expandEntry(e); ok {
			out = append(out, abs)
		}
	}
	return out
}

// MentionsDenied reports whether free text (a shell command, a script passed
// to an interpreter) names a deny-read credential store, in absolute form or
// as ~/..., $HOME/..., %USERPROFILE%\... . It catches literals that token
// parsing cannot see, such as a path inside python -c "open('~/.aws/credentials')".
// It returns the matching entry. Matching ignores case and quote characters
// and treats both slash kinds alike; a hit must end at a path boundary, so
// ".aws" does not match ".awsome".
func (p Policy) MentionsDenied(text string) (entry string, found bool) {
	if text == "" || len(p.DenyRead) == 0 {
		return "", false
	}
	goos := p.os()
	norm := strings.ToLower(strings.NewReplacer(`\`, "/", `"`, "", `'`, "").Replace(text))
	homeKey := ""
	if h := p.home(); h != "" {
		homeKey = strings.ToLower(KeyOS("linux", strings.ReplaceAll(h, `\`, "/")))
	}
	for _, e := range p.DenyRead {
		abs, ok := p.expandEntry(e)
		if !ok {
			continue
		}
		key := strings.ToLower(strings.ReplaceAll(CleanOS(goos, abs), `\`, "/"))
		needles := []string{key}
		if homeKey != "" && strings.HasPrefix(key, homeKey+"/") {
			rel := key[len(homeKey)+1:]
			for _, pre := range []string{"~/", "$home/", "${home}/", "%userprofile%/", "$env:userprofile/", "$env:home/", "$userprofile/"} {
				needles = append(needles, pre+rel)
			}
			for _, sub := range []struct{ dir, a, b string }{
				{"appdata/local/", "%localappdata%/", "$env:localappdata/"},
				{"appdata/roaming/", "%appdata%/", "$env:appdata/"},
			} {
				if strings.HasPrefix(rel, sub.dir) {
					needles = append(needles, sub.a+rel[len(sub.dir):], sub.b+rel[len(sub.dir):])
				}
			}
		}
		for _, n := range needles {
			if containsPath(norm, n) {
				return abs, true
			}
		}
	}
	return "", false
}

// containsPath finds needle in text where the match starts and ends at a path
// boundary.
func containsPath(text, needle string) bool {
	from := 0
	for {
		i := strings.Index(text[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(needle)
		startOK := i == 0 || !isNameByte(text[i-1]) && text[i-1] != '/'
		endOK := end == len(text) || !isNameByte(text[end])
		if startOK && endOK {
			return true
		}
		from = i + 1
	}
}

func isNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_' || b == '-' || b == '.'
}
