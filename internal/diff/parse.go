package diff

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/vcs"
)

// parseState is the per-file bookkeeping that is not part of the model.
type parseState struct {
	newFile, deleted bool
	renameFrom       string
	renameTo         string
	sawOld           bool // saw a "--- " line
}

// Parse parses `git diff` unified output (with a/ b/ prefixes). It also
// accepts plain `diff -u` output without "diff --git" lines. Parse never
// panics; it returns an error only for a header it cannot make sense of, in
// which case the files parsed so far are returned with it.
//
// Hunk bodies are read using the line counts in the @@ header, so content
// lines that look like headers ("--- x", "diff --git ...") are handled
// correctly. Text fields are substrings of raw, so raw stays alive as long as
// the result does.
func Parse(raw string) ([]File, error) {
	if raw == "" {
		return nil, nil
	}
	// One arena for all lines: hunks get capacity-clipped sub-slices, so a
	// parse costs a single allocation for every Line in the diff.
	arena := make([]Line, 0, strings.Count(raw, "\n")+1)
	var (
		files        []File
		st           parseState
		cur          *File
		inHunk       bool
		remOld       int
		remNew       int
		hunkStart    int // index into arena where the current hunk began
		skipBinary   bool
		lineNo       int
		prevLine     *Line
		pos          int
		finishedFile = func() {
			if cur != nil {
				finishFile(cur, &st)
			}
		}
		endHunk = func() {
			if inHunk && cur != nil {
				h := &cur.Hunks[len(cur.Hunks)-1]
				h.Lines = arena[hunkStart:len(arena):len(arena)]
			}
			inHunk = false
			prevLine = nil
		}
	)
	for pos < len(raw) {
		var line string
		if i := strings.IndexByte(raw[pos:], '\n'); i >= 0 {
			line = raw[pos : pos+i]
			pos += i + 1
		} else {
			line = raw[pos:]
			pos = len(raw)
		}
		lineNo++

		if inHunk {
			if remOld > 0 || remNew > 0 {
				var kind LineKind
				ok := true
				text := ""
				if len(line) == 0 {
					// Some tools strip the single space of an empty context line.
					kind = Context
				} else {
					text = line[1:]
					switch line[0] {
					case ' ':
						kind = Context
					case '-':
						kind = Del
					case '+':
						kind = Add
					case '\\':
						if prevLine != nil {
							prevLine.NoNewlineAtEOF = true
						}
						continue
					default:
						ok = false
					}
				}
				if ok {
					switch kind {
					case Context:
						ok = remOld > 0 && remNew > 0
					case Del:
						ok = remOld > 0
					case Add:
						ok = remNew > 0
					}
				}
				if ok {
					h := &cur.Hunks[len(cur.Hunks)-1]
					var l Line
					l.Kind = kind
					if n := len(text); n > 0 && text[n-1] == '\r' {
						text = text[:n-1]
						l.CRLF = true
					}
					l.Text = text
					switch kind {
					case Context:
						l.OldNo = h.OldStart + h.OldLines - remOld
						l.NewNo = h.NewStart + h.NewLines - remNew
						remOld--
						remNew--
					case Del:
						l.OldNo = h.OldStart + h.OldLines - remOld
						remOld--
						cur.Deleted++
					case Add:
						l.NewNo = h.NewStart + h.NewLines - remNew
						remNew--
						cur.Added++
					}
					arena = append(arena, l)
					prevLine = &arena[len(arena)-1]
					continue
				}
				// Malformed hunk body: close it and treat the line as a header.
				endHunk()
			} else if len(line) > 0 && line[0] == '\\' {
				if prevLine != nil {
					prevLine.NoNewlineAtEOF = true
				}
				continue
			} else {
				endHunk()
			}
		}

		line = strings.TrimSuffix(line, "\r")
		if skipBinary {
			if !strings.HasPrefix(line, "diff --git ") {
				continue
			}
			skipBinary = false
		}

		switch {
		case line == "diff --git" || strings.HasPrefix(line, "diff --git "):
			finishedFile()
			oldP, newP, ok := splitGitHeader(strings.TrimPrefix(line[len("diff --git"):], " "))
			if !ok {
				return files, fmt.Errorf("diff: line %d: cannot parse header %q", lineNo, line)
			}
			files = append(files, File{OldPath: oldP, NewPath: newP, Status: vcs.Modified})
			cur = &files[len(files)-1]
			st = parseState{}

		case strings.HasPrefix(line, "--- ") && nextIsPlus(raw[pos:]) &&
			(cur == nil || st.sawOld || len(cur.Hunks) > 0):
			// Plain unified diff without a "diff --git" line.
			finishedFile()
			files = append(files, File{Status: vcs.Modified})
			cur = &files[len(files)-1]
			st = parseState{}
			cur.OldPath = diffPath(line[4:], "a/")
			st.sawOld = true
			if cur.OldPath == "" {
				st.newFile = true
			}

		case cur == nil:
			// Preamble (commit message, mail headers...): ignore.

		case strings.HasPrefix(line, "@@"):
			h, ok := parseHunkHeader(line)
			if !ok {
				finishedFile()
				return files, fmt.Errorf("diff: line %d: bad hunk header %q", lineNo, line)
			}
			cur.Hunks = append(cur.Hunks, h)
			inHunk = true
			remOld, remNew = h.OldLines, h.NewLines
			hunkStart = len(arena)

		case strings.HasPrefix(line, "--- "):
			st.sawOld = true
			p := diffPath(line[4:], "a/")
			cur.OldPath = p
			if p == "" {
				st.newFile = true
			}
		case strings.HasPrefix(line, "+++ "):
			p := diffPath(line[4:], "b/")
			cur.NewPath = p
			if p == "" {
				st.deleted = true
			}
		case strings.HasPrefix(line, "new file mode "):
			st.newFile = true
			cur.NewMode = line[len("new file mode "):]
		case strings.HasPrefix(line, "deleted file mode "):
			st.deleted = true
			cur.OldMode = line[len("deleted file mode "):]
		case strings.HasPrefix(line, "old mode "):
			cur.OldMode = line[len("old mode "):]
		case strings.HasPrefix(line, "new mode "):
			cur.NewMode = line[len("new mode "):]
		case strings.HasPrefix(line, "similarity index "):
			s := strings.TrimSuffix(line[len("similarity index "):], "%")
			cur.Similarity, _ = strconv.Atoi(s)
		case strings.HasPrefix(line, "rename from "):
			st.renameFrom = unquoteMaybe(line[len("rename from "):])
		case strings.HasPrefix(line, "rename to "):
			st.renameTo = unquoteMaybe(line[len("rename to "):])
		case strings.HasPrefix(line, "Binary files "), line == "GIT binary patch":
			cur.Binary = true
			if line == "GIT binary patch" {
				skipBinary = true
			}
		}
	}
	endHunk()
	finishedFile()
	return files, nil
}

// finishFile settles Status and the paths once all header lines are known.
func finishFile(f *File, st *parseState) {
	if st.renameFrom != "" || st.renameTo != "" {
		if st.renameFrom != "" {
			f.OldPath = st.renameFrom
		}
		if st.renameTo != "" {
			f.NewPath = st.renameTo
		}
	}
	switch {
	case st.newFile:
		f.Status = vcs.Added
		f.OldPath = ""
	case st.deleted:
		f.Status = vcs.Deleted
		f.NewPath = ""
	case st.renameFrom != "" || st.renameTo != "":
		f.Status = vcs.Renamed
	default:
		f.Status = vcs.Modified
	}
	if f.NewPath == "" && f.Status != vcs.Deleted {
		f.NewPath = f.OldPath
	}
	if f.OldPath == "" && f.Status != vcs.Added {
		f.OldPath = f.NewPath
	}
}

// nextIsPlus reports whether the next line of rest starts with "+++ ".
func nextIsPlus(rest string) bool { return strings.HasPrefix(rest, "+++ ") }

// unquoteMaybe unquotes a git path that may be C-quoted.
func unquoteMaybe(s string) string {
	if strings.HasPrefix(s, "\"") {
		if v, _, ok := unquoteC(s); ok {
			return v
		}
	}
	return s
}

// diffPath turns the operand of a "--- " / "+++ " line into a path: /dev/null
// becomes "", quoting and the trailing tab git adds after paths with spaces
// are removed, and the a/ or b/ prefix is stripped.
func diffPath(s, prefix string) string {
	if strings.HasPrefix(s, "\"") {
		if v, _, ok := unquoteC(s); ok {
			s = v
		}
	} else if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	if s == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(s, prefix)
}

// splitGitHeader splits the operand of a "diff --git" line into the old and
// new paths. Paths may contain spaces and may be C-quoted, so the split is
// ambiguous in general; when both sides name the same file (everything except
// renames) the split where the halves agree wins. Renames get their real
// paths from the "rename from/to" lines later.
func splitGitHeader(s string) (oldP, newP string, ok bool) {
	if s == "" {
		return "", "", false
	}
	if s[0] == '"' {
		o, rest, ok := unquoteC(s)
		if !ok {
			return "", "", false
		}
		rest = strings.TrimPrefix(rest, " ")
		var n string
		if strings.HasPrefix(rest, "\"") {
			n, _, ok = unquoteC(rest)
			if !ok {
				return "", "", false
			}
		} else {
			n = rest
		}
		return strings.TrimPrefix(o, "a/"), strings.TrimPrefix(n, "b/"), n != ""
	}
	first := -1
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' || i+1 >= len(s) {
			continue
		}
		rest := s[i+1:]
		var n string
		switch {
		case strings.HasPrefix(rest, "b/"):
			n = rest[2:]
		case strings.HasPrefix(rest, "\"b/") || strings.HasPrefix(rest, "\""):
			v, _, ok := unquoteC(rest)
			if !ok {
				continue
			}
			n = strings.TrimPrefix(v, "b/")
		default:
			continue
		}
		o := strings.TrimPrefix(s[:i], "a/")
		if o == n {
			return o, n, true
		}
		if first < 0 {
			first = i
		}
	}
	if first >= 0 {
		rest := s[first+1:]
		n := rest
		if strings.HasPrefix(rest, "\"") {
			if v, _, ok := unquoteC(rest); ok {
				n = v
			}
		}
		return strings.TrimPrefix(s[:first], "a/"), strings.TrimPrefix(n, "b/"), true
	}
	// No prefixes (--no-prefix): accept "X X".
	if len(s)%2 == 1 {
		m := len(s) / 2
		if s[m] == ' ' && s[:m] == s[m+1:] {
			return s[:m], s[m+1:], true
		}
	}
	return "", "", false
}

// parseHunkHeader parses "@@ -a[,b] +c[,d] @@ header".
func parseHunkHeader(line string) (Hunk, bool) {
	var h Hunk
	if !strings.HasPrefix(line, "@@ -") {
		return h, false
	}
	s := line[4:]
	var ok bool
	if h.OldStart, h.OldLines, s, ok = parseRange(s); !ok {
		return h, false
	}
	if !strings.HasPrefix(s, " +") {
		return h, false
	}
	if h.NewStart, h.NewLines, s, ok = parseRange(s[2:]); !ok {
		return h, false
	}
	if !strings.HasPrefix(s, " @@") {
		return h, false
	}
	h.Header = strings.TrimPrefix(strings.TrimPrefix(s[3:], " "), "\t")
	h.Header = strings.TrimRight(h.Header, "\r")
	return h, true
}

// parseRange parses "start[,count]" and returns the text after it.
func parseRange(s string) (start, count int, rest string, ok bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 9 {
		return 0, 0, s, false
	}
	start, _ = strconv.Atoi(s[:i])
	s = s[i:]
	count = 1
	if len(s) > 0 && s[0] == ',' {
		s = s[1:]
		j := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == 0 || j > 9 {
			return 0, 0, s, false
		}
		count, _ = strconv.Atoi(s[:j])
		s = s[j:]
	}
	return start, count, s, true
}
