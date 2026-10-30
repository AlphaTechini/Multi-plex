package diff

import "github.com/alphatechini/multi-plex/internal/vcs"

// LineKind says how a diff line relates to the two sides.
type LineKind uint8

// Line kinds.
const (
	// Context is an unchanged line present on both sides.
	Context LineKind = iota
	// Add is a line present only on the new side.
	Add
	// Del is a line present only on the old side.
	Del
)

// File is the parsed diff of one file.
//
// Path semantics follow git's /dev/null convention: OldPath is empty for an
// added file and NewPath is empty for a deleted file. Use Path for display.
// Paths are slash-separated, relative to the repository root, without the
// a/ b/ prefixes.
type File struct {
	OldPath string
	NewPath string
	Status  vcs.FileStatus
	Binary  bool
	Hunks   []Hunk
	Added   int // number of Add lines
	Deleted int // number of Del lines

	// OldMode and NewMode are the git file modes ("100644") when the diff
	// states them. A mode-only change has differing modes and no hunks.
	OldMode string
	NewMode string
	// Similarity is the rename similarity percentage (0 when not a rename).
	Similarity int
}

// Path returns the path to show for the file: NewPath, or OldPath for a
// deleted file.
func (f *File) Path() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
}

// ModeOnly reports whether the file changed only its mode.
func (f *File) ModeOnly() bool {
	return len(f.Hunks) == 0 && !f.Binary && f.Status == vcs.Modified &&
		f.OldMode != "" && f.NewMode != "" && f.OldMode != f.NewMode
}

// Hunk is one @@ block.
type Hunk struct {
	// OldStart/OldLines and NewStart/NewLines are the numbers from the @@
	// header. As in git, the start of an empty range is the line before it.
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	// Header is the text after the closing @@ (usually function context),
	// without the leading space.
	Header string
	Lines  []Line
}

// Line is one line of a hunk.
//
// Line endings: Text never contains the trailing "\n" nor a trailing "\r".
// A line that ended in "\r\n" has CRLF set, so no information is lost and
// renderers never see a stray carriage return.
type Line struct {
	Kind LineKind
	// OldNo and NewNo are 1-based line numbers, 0 when the line is absent on
	// that side (OldNo for Add lines, NewNo for Del lines).
	OldNo int
	NewNo int
	// Text is the line without the +/-/space prefix and line ending.
	Text string
	// Segments is nil unless the line was word-diffed (see ApplyWordDiff).
	// When set, the segments are contiguous, cover Text completely and
	// alternate between unchanged and changed. A nil Segments on an Add or
	// Del line means "highlight the whole line".
	Segments []Segment
	// NoNewlineAtEOF is set when git reported "\ No newline at end of file"
	// for this line.
	NoNewlineAtEOF bool
	// CRLF is set when the line ended in "\r\n".
	CRLF bool
	// Expanded marks context lines inserted by ExpandGap; Rows never
	// collapses them again.
	Expanded bool
}

// Segment is a byte range of a Line's Text.
type Segment struct {
	Start, End int // byte offsets into Text, End exclusive
	Changed    bool
}
