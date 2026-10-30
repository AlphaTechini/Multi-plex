package gitcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
)

// MergeConflicts returns the files that would conflict if a and b were merged,
// using git merge-tree --write-tree (Git 2.38 or newer). No worktree or index
// is touched; git only writes the merged tree object. A clean merge returns
// nil.
func (g *Git) MergeConflicts(ctx context.Context, repo, a, b string) ([]string, error) {
	if err := checkArg("revision", a); err != nil {
		return nil, err
	}
	if err := checkArg("revision", b); err != nil {
		return nil, err
	}
	ok, err := g.versionAtLeast(ctx, 2, 38)
	if err != nil {
		return nil, err
	}
	if !ok {
		v, _ := g.Version(ctx)
		return nil, fmt.Errorf("gitcli: conflict prediction needs git 2.38 or newer (merge-tree --write-tree), found %q", v)
	}
	out, err := g.run(ctx, runOpts{dir: repo}, "merge-tree", "--write-tree", "--name-only", "--no-messages", "-z", a, b)
	if err == nil {
		return nil, nil
	}
	if exitCode(err) != 1 {
		return nil, err
	}
	// Exit 1 means conflicts, but git also uses 1 for unknown revisions, so
	// the output must begin with a tree id.
	toks := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(toks) == 0 || !isOID(toks[0]) {
		var ge *Error
		if errors.As(err, &ge) && ge.Stderr != "" {
			return nil, err
		}
		return nil, fmt.Errorf("gitcli: merge-tree: unexpected output %q", out)
	}
	files := make([]string, 0, len(toks)-1)
	for _, t := range toks[1:] {
		if len(t) > 0 {
			files = append(files, string(t))
		}
	}
	sort.Strings(files)
	return files, nil
}

// isOID reports whether b is a full hex object id (SHA-1 or SHA-256).
func isOID(b []byte) bool {
	if len(b) != 40 && len(b) != 64 {
		return false
	}
	for _, c := range b {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
