package classify

import "strings"

var gitGlobalValue = setOf("-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--super-prefix", "--config-env")

// normalizeGit removes git's global options so rules can match on the
// subcommand ("git -c x=y -C dir push -f" becomes "git push -f"). It returns
// the directory given by -C or --work-tree, if any, so path checks use it.
func normalizeGit(argv []string) (out []string, dir string) {
	out = append(out, argv[0])
	i := 1
	for i < len(argv) {
		a := argv[i]
		if len(a) < 2 || a[0] != '-' {
			break
		}
		switch {
		case a == "-C" && i+1 < len(argv):
			dir = joinDir(dir, argv[i+1])
			i += 2
		case a == "--work-tree" && i+1 < len(argv):
			dir = argv[i+1]
			i += 2
		case strings.HasPrefix(a, "--work-tree="):
			dir = strings.TrimPrefix(a, "--work-tree=")
			i++
		case gitGlobalValue[a] && i+1 < len(argv):
			i += 2
		default:
			i++ // --no-pager, -P, --bare, --git-dir=..., -cfoo=bar ...
		}
	}
	return append(out, argv[i:]...), dir
}

// joinDir combines successive -C arguments (each relative to the previous).
func joinDir(prev, next string) string {
	if prev == "" || strings.HasPrefix(next, "/") || strings.HasPrefix(next, `\`) || (len(next) > 1 && next[1] == ':') || strings.HasPrefix(next, "~") {
		return next
	}
	return prev + "/" + next
}
