package diffview

import (
	"hash/fnv"

	"github.com/alphatechini/multi-plex/internal/diff"
)

// groupKey identifies a run of changed lines of a file by its content.
type groupKey struct {
	path string
	hash uint64
}

// reviewState is what the reviewer has marked as read in one session. It is
// kept in memory per session. A file counts as reviewed only while its diff
// is unchanged (the key includes the diff's hash), so new edits reopen it.
// A hunk is reviewed when every run of changed lines in it was marked, which
// survives expanding context (hunks merging).
type reviewState struct {
	files  map[string]uint64
	groups map[groupKey]struct{}
}

func newReviewState() *reviewState {
	return &reviewState{files: map[string]uint64{}, groups: map[groupKey]struct{}{}}
}

// fileReviewed reports whether the file is marked and unchanged since.
func (r *reviewState) fileReviewed(e *fileEntry) bool {
	h, ok := r.files[e.path]
	return ok && h == e.hash
}

// toggleFile flips a file's reviewed mark.
func (r *reviewState) toggleFile(e *fileEntry) {
	if r.fileReviewed(e) {
		delete(r.files, e.path)
		return
	}
	r.files[e.path] = e.hash
}

// changeGroups returns the content hash of each run of added/deleted lines
// of a hunk.
func changeGroups(h *diff.Hunk) []uint64 {
	var out []uint64
	for i := 0; i < len(h.Lines); {
		if h.Lines[i].Kind == diff.Context {
			i++
			continue
		}
		g := fnv.New64a()
		for i < len(h.Lines) && h.Lines[i].Kind != diff.Context {
			_, _ = g.Write([]byte{byte(h.Lines[i].Kind)})
			_, _ = g.Write([]byte(h.Lines[i].Text))
			_, _ = g.Write([]byte{0})
			i++
		}
		out = append(out, g.Sum64())
	}
	return out
}

// hunkReviewed reports whether every change group of the hunk is marked.
func (r *reviewState) hunkReviewed(path string, h *diff.Hunk) bool {
	groups := changeGroups(h)
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if _, ok := r.groups[groupKey{path, g}]; !ok {
			return false
		}
	}
	return true
}

// toggleHunk flips the reviewed mark of a hunk.
func (r *reviewState) toggleHunk(path string, h *diff.Hunk) {
	groups := changeGroups(h)
	if r.hunkReviewed(path, h) {
		for _, g := range groups {
			delete(r.groups, groupKey{path, g})
		}
		return
	}
	for _, g := range groups {
		r.groups[groupKey{path, g}] = struct{}{}
	}
}

// reviewedCount returns how many of the files are reviewed.
func (r *reviewState) reviewedCount(files []*fileEntry) int {
	n := 0
	for _, e := range files {
		if r.fileReviewed(e) {
			n++
		}
	}
	return n
}
