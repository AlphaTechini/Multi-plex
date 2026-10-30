package claudecode

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
)

// maxEditFileSize caps how large a file is read to locate an edit.
const maxEditFileSize = 2 << 20

// maxRangesPerEdit caps the ranges reported for a replace_all edit.
const maxRangesPerEdit = 20

// editRanges works out which lines a finished Edit, MultiEdit or Write
// touched, for follow mode. It is cheap by design: Write is the whole file,
// and for Edit the new text is searched for in the file on disk (cwd-relative
// paths are resolved against cwd; files over 2 MB are skipped). When the text
// cannot be found, for example a pure deletion or a file changed since, no
// range is reported; the event's Paths still name the file.
func editRanges(tool string, input map[string]any, cwd string) []agent.EditRange {
	path, _ := input["file_path"].(string)
	if path == "" {
		return nil
	}
	switch tool {
	case "Write":
		content, _ := input["content"].(string)
		return []agent.EditRange{{Path: path, StartLine: 1, EndLine: max(1, lineCount(content))}}
	case "Edit":
		newStr, _ := input["new_string"].(string)
		all, _ := input["replace_all"].(bool)
		return locate(path, cwd, []string{newStr}, all)
	case "MultiEdit":
		edits, _ := input["edits"].([]any)
		var news []string
		for _, e := range edits {
			if m, ok := e.(map[string]any); ok {
				if s, _ := m["new_string"].(string); s != "" {
					news = append(news, s)
				}
			}
		}
		return locate(path, cwd, news, false)
	}
	return nil
}

// locate finds each text in the file and returns its line ranges.
func locate(path, cwd string, texts []string, all bool) []agent.EditRange {
	file := path
	if !filepath.IsAbs(file) {
		file = filepath.Join(cwd, file)
	}
	data, ok := readSmallFile(file)
	if !ok {
		return nil
	}
	crlf := bytes.Contains(data, []byte("\r\n"))
	content := string(data)

	var out []agent.EditRange
	for _, text := range texts {
		if text == "" {
			continue
		}
		if crlf {
			// The model writes \n; the file on Windows may use \r\n.
			text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
		}
		from := 0
		for n := 0; n < maxRangesPerEdit; n++ {
			i := strings.Index(content[from:], text)
			if i < 0 {
				break
			}
			i += from
			start := 1 + strings.Count(content[:i], "\n")
			out = append(out, agent.EditRange{Path: path, StartLine: start, EndLine: start + max(0, lineCount(text)-1)})
			if !all {
				break
			}
			from = i + len(text)
		}
	}
	return out
}

// lineCount returns the number of lines in s: a trailing newline does not
// start another line.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// readSmallFile reads a regular file of at most maxEditFileSize bytes.
func readSmallFile(path string) ([]byte, bool) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() > maxEditFileSize {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}
