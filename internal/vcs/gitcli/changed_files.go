package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/vcs"
)

// ChangedFiles lists files changed in the worktree against the merge base of
// base and HEAD, including untracked non-ignored files, sorted by path. Paths
// are slash-separated and relative to the worktree root.
func (g *Git) ChangedFiles(ctx context.Context, worktree, base string) ([]vcs.FileChange, error) {
	mb, err := g.diffBase(ctx, worktree, base)
	if err != nil {
		return nil, err
	}
	var out []byte
	err = g.withTempIndex(ctx, worktree, func(o runOpts) error {
		o.readOnly = true
		out, err = g.run(ctx, o, "diff", "--cached", "--raw", "--numstat", "-z", "-M",
			"--no-color", "--no-ext-diff", "--no-textconv", mb)
		return err
	})
	if err != nil {
		return nil, err
	}
	return parseChanges(out)
}

// parseChanges parses `git diff --raw --numstat -z` output: first every raw
// record (":modes oids STATUS" NUL path [NUL newpath] NUL), then every numstat
// record ("added TAB deleted TAB path" NUL, or for renames "added TAB deleted
// TAB" NUL old NUL new NUL).
func parseChanges(out []byte) ([]vcs.FileChange, error) {
	toks := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(toks) == 1 && len(toks[0]) == 0 {
		return nil, nil
	}
	var files []vcs.FileChange
	byPath := map[string]int{}
	i := 0
	for ; i < len(toks) && bytes.HasPrefix(toks[i], []byte(":")); i++ {
		meta := strings.Fields(string(toks[i]))
		if len(meta) < 5 {
			return nil, fmt.Errorf("gitcli: bad raw diff record %q", toks[i])
		}
		st := meta[4]
		var fc vcs.FileChange
		switch st[0] {
		case 'A':
			fc.Status = vcs.Added
		case 'D':
			fc.Status = vcs.Deleted
		case 'R', 'C':
			fc.Status = vcs.Renamed
		default: // M, T
			fc.Status = vcs.Modified
		}
		n := 1
		if st[0] == 'R' || st[0] == 'C' {
			n = 2
		}
		if i+n >= len(toks) {
			return nil, fmt.Errorf("gitcli: truncated raw diff output")
		}
		if n == 2 {
			fc.OldPath = string(toks[i+1])
			fc.Path = string(toks[i+2])
		} else {
			fc.Path = string(toks[i+1])
		}
		i += n
		byPath[fc.Path] = len(files)
		files = append(files, fc)
	}
	for ; i < len(toks); i++ {
		parts := strings.SplitN(string(toks[i]), "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("gitcli: bad numstat record %q", toks[i])
		}
		path := parts[2]
		if path == "" { // rename: old and new follow as separate tokens
			if i+2 >= len(toks) {
				return nil, fmt.Errorf("gitcli: truncated numstat output")
			}
			path = string(toks[i+2])
			i += 2
		}
		idx, ok := byPath[path]
		if !ok {
			continue
		}
		if parts[0] == "-" || parts[1] == "-" {
			files[idx].Binary = true
			continue
		}
		files[idx].Added, _ = strconv.Atoi(parts[0])
		files[idx].Deleted, _ = strconv.Atoi(parts[1])
	}
	sort.Slice(files, func(a, b int) bool { return files[a].Path < files[b].Path })
	return files, nil
}
