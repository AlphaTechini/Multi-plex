package diffview

import (
	"hash/fnv"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// fileEntry is one changed file of the diff, in rail order.
type fileEntry struct {
	file diff.File
	path string
	dir  string // "" for files in the repository root
	name string
	// hash identifies the file's diff as parsed (it does not change when
	// regions are expanded); reviewed state is keyed by it.
	hash uint64
	// hunkHashes are per-hunk hashes of the parsed diff, to spot new hunks.
	hunkHashes []uint64
	// huge files are not opened until the user asks.
	huge bool
}

// status is the word shown in the banner.
func (e *fileEntry) status() string {
	switch e.file.Status {
	case vcs.Added:
		return "added"
	case vcs.Deleted:
		return "deleted"
	case vcs.Renamed:
		return "renamed"
	}
	return "modified"
}

// letter is the one-letter status shown in the rail.
func (e *fileEntry) letter() string {
	switch e.file.Status {
	case vcs.Added:
		return "A"
	case vcs.Deleted:
		return "D"
	case vcs.Renamed:
		return "R"
	}
	return "M"
}

// newEntry wraps a parsed file.
func newEntry(f diff.File) *fileEntry {
	e := &fileEntry{file: f, path: f.Path()}
	dir, name := path.Split(e.path)
	e.dir = strings.TrimSuffix(dir, "/")
	e.name = name
	e.hash, e.hunkHashes = hashFile(&e.file)
	e.huge = f.Added+f.Deleted > hugeDiffLines
	return e
}

// sortEntries orders files by directory, then name: the rail order, which is
// also the order of the next and previous file keys.
func sortEntries(files []*fileEntry) {
	sort.SliceStable(files, func(i, j int) bool {
		a, b := files[i], files[j]
		if a.dir != b.dir {
			return a.dir < b.dir
		}
		return a.name < b.name
	})
}

// hashFile hashes a parsed file and each of its hunks. Line numbers are left
// out of the hunk hashes so a hunk that only moved is not new.
func hashFile(f *diff.File) (uint64, []uint64) {
	h := fnv.New64a()
	hunks := make([]uint64, len(f.Hunks))
	write := func(h interface{ Write([]byte) (int, error) }, s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	write(h, f.Path())
	write(h, string(f.Status))
	write(h, f.OldMode+">"+f.NewMode)
	if f.Binary {
		write(h, "binary")
	}
	for i := range f.Hunks {
		hh := fnv.New64a()
		for j := range f.Hunks[i].Lines {
			l := &f.Hunks[i].Lines[j]
			_, _ = hh.Write([]byte{byte(l.Kind)})
			_, _ = hh.Write([]byte(l.Text))
			_, _ = hh.Write([]byte{0})
		}
		hunks[i] = hh.Sum64()
		write(h, strconv.FormatUint(hunks[i], 16))
		write(h, strconv.Itoa(f.Hunks[i].OldStart)+","+strconv.Itoa(f.Hunks[i].NewStart))
	}
	return h.Sum64(), hunks
}
