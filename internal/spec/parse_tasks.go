package spec

import (
	"regexp"
	"strconv"
	"strings"
)

// Task is one checkbox item in tasks.md.
type Task struct {
	// ID is the leading number ("1", "1.2") or, without one, the task's 1-based position.
	ID string
	// Title is the item text without the leading number or checkbox.
	Title string
	// Done is true for "[x]" or "[X]".
	Done bool
	// InProgress is true for "[~]" or "[-]".
	InProgress bool
	// Requirements lists the requirement IDs (R1.2 form) cited by the task,
	// including its own nested lines such as "_Requirements: R1.1_".
	Requirements []string
	// Line is the 1-based line number of the checkbox.
	Line int
}

// taskRe matches a checkbox line. Group 1 is indentation, group 2 the marker
// character, group 3 the item text. A "*" after the box marks an optional task.
var taskRe = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX~\-])\]\*?\s*(.*?)\s*$`)

// numberRe splits a leading "1", "1.2" or "1." from the task text.
var numberRe = regexp.MustCompile(`^(\d+(?:\.\d+)*)\.?\s+(.*)$`)

// requirementRe matches a requirement ID such as R1.2. Go's \b is not used
// because "_" counts as a word character, and "_Requirements: R1.1_" is common.
var requirementRe = regexp.MustCompile(`R\d+\.\d+`)

// ParseTasks returns every checkbox item in md, including nested ones, in file order.
func ParseTasks(md string) []Task {
	lines := strings.Split(md, "\n")
	var tasks []Task
	for i, line := range lines {
		m := taskRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		t := Task{
			Done:       m[2] == "x" || m[2] == "X",
			InProgress: m[2] == "~" || m[2] == "-",
			Line:       i + 1,
		}
		if n := numberRe.FindStringSubmatch(m[3]); n != nil {
			t.ID, t.Title = n[1], n[2]
		} else {
			t.ID, t.Title = strconv.Itoa(len(tasks)+1), m[3]
		}
		t.Requirements = taskRequirements(lines, i, len(m[1]), m[3])
		tasks = append(tasks, t)
	}
	return tasks
}

// taskRequirements collects requirement IDs from a task's line and from the
// lines indented under it. The scan stops at the next checkbox or at a
// non-blank line that is not indented deeper than the task.
func taskRequirements(lines []string, at, indent int, text string) []string {
	var ids []string
	seen := map[string]bool{}
	add := func(s string) {
		for _, id := range requirementIDs(s) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	add(text)
	for _, line := range lines[at+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if taskRe.MatchString(line) || leadingSpace(line) <= indent {
			break
		}
		add(line)
	}
	return ids
}

// leadingSpace returns the width of the line's leading whitespace.
func leadingSpace(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// requirementIDs returns the requirement IDs in s. An ID must not be glued to
// a letter or digit on either side, so "XR1.1" and "R1.10x" do not match.
func requirementIDs(s string) []string {
	var ids []string
	for _, loc := range requirementRe.FindAllStringIndex(s, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && isAlnum(s[start-1]) {
			continue
		}
		if end < len(s) && (isAlnum(s[end]) || (s[end] == '.' && end+1 < len(s) && isDigit(s[end+1]))) {
			continue
		}
		ids = append(ids, s[start:end])
	}
	return ids
}

// isAlnum reports whether b is an ASCII letter or digit.
func isAlnum(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// isDigit reports whether b is an ASCII digit.
func isDigit(b byte) bool { return b >= '0' && b <= '9' }
