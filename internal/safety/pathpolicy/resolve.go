package pathpolicy

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const maxSymlinks = 40

// resolveSymlinks resolves symlinks in the absolute host path p component by
// component, so "link/../x" is resolved the way the OS would (parent of the
// link target, not of the link). Components that do not exist yet are kept as
// given, which is what a file about to be created looks like. Errors other
// than "does not exist" (loops, permissions) are returned: the caller must
// treat the path as unresolvable and gate it.
func resolveSymlinks(p string) (string, error) {
	vol := filepath.VolumeName(p)
	root := vol + string(filepath.Separator)
	queue := splitComponents(p[len(vol):])
	cur := root
	links := 0
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			cur = parentOf(cur, root)
			continue
		}
		next := filepath.Join(cur, c)
		fi, err := os.Lstat(next)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// The rest cannot be symlinks; apply it lexically.
				rest := make([]string, 0, len(queue)+1)
				rest = append(rest, c)
				rest = append(rest, queue...)
				for _, r := range rest {
					switch r {
					case "", ".":
					case "..":
						cur = parentOf(cur, root)
					default:
						cur = filepath.Join(cur, r)
					}
				}
				return cur, nil
			}
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			cur = next
			continue
		}
		links++
		if links > maxSymlinks {
			return "", errors.New("too many symbolic links")
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if runtime.GOOS == "windows" {
			target = strings.TrimPrefix(target, `\??\`)
			target = strings.TrimPrefix(target, `\\?\`)
		}
		if filepath.IsAbs(target) || strings.HasPrefix(target, string(filepath.Separator)) {
			tv := filepath.VolumeName(target)
			if tv == "" {
				tv = vol
			}
			cur = tv + string(filepath.Separator)
			root = cur
			queue = append(splitComponents(target[len(filepath.VolumeName(target)):]), queue...)
		} else {
			queue = append(splitComponents(target), queue...)
		}
	}
	return cur, nil
}

func splitComponents(s string) []string {
	if runtime.GOOS == "windows" {
		s = strings.ReplaceAll(s, "/", `\`)
		return strings.Split(s, `\`)
	}
	return strings.Split(s, "/")
}

func parentOf(cur, root string) string {
	if cur == root {
		return cur
	}
	d := filepath.Dir(cur)
	if len(d) < len(root) {
		return root
	}
	return d
}

// Policy entries (the worktree, deny-read stores) are checked on every call
// and almost never change, so their resolved forms are cached briefly.
const entryCacheTTL = 5 * time.Second

var entryCache = struct {
	sync.Mutex
	m map[string]cachedEntry
}{m: map[string]cachedEntry{}}

type cachedEntry struct {
	path    string
	err     error
	expires time.Time
}

func resolveEntryCached(p string) (string, error) {
	now := time.Now()
	entryCache.Lock()
	if e, ok := entryCache.m[p]; ok && now.Before(e.expires) {
		entryCache.Unlock()
		return e.path, e.err
	}
	entryCache.Unlock()
	r, err := resolveSymlinks(p)
	entryCache.Lock()
	if len(entryCache.m) > 1024 {
		entryCache.m = map[string]cachedEntry{}
	}
	entryCache.m[p] = cachedEntry{path: r, err: err, expires: now.Add(entryCacheTTL)}
	entryCache.Unlock()
	return r, err
}
