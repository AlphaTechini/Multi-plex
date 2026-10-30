package fake

import (
	"fmt"
	"strings"
)

// hunk is one @@ block of the fake diff. Lines start with ' ', '+' or '-'.
type hunk struct {
	oldStart, newStart int
	section            string
	lines              []string
}

// fileDiff is one file of the fake diff.
type fileDiff struct {
	path, oldPath string // oldPath is set for renames
	status        string // "M", "A", "R"
	hunks         []hunk
}

func (h hunk) counts() (oldN, newN int) {
	for _, l := range h.lines {
		switch l[0] {
		case ' ':
			oldN++
			newN++
		case '-':
			oldN++
		case '+':
			newN++
		}
	}
	return
}

// raw renders the file as `git diff --histogram` would.
func (f fileDiff) raw() string {
	var b strings.Builder
	old := f.oldPath
	if old == "" {
		old = f.path
	}
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", old, f.path)
	switch f.status {
	case "A":
		b.WriteString("new file mode 100644\nindex 0000000..3b18e51\n--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", f.path)
	case "R":
		b.WriteString("similarity index 82%\n")
		fmt.Fprintf(&b, "rename from %s\nrename to %s\nindex 5d1a9c2..8e07b34 100644\n", old, f.path)
		fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", old, f.path)
	default:
		b.WriteString("index 9f3b1a7..c2d84e0 100644\n")
		fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", old, f.path)
	}
	for _, h := range f.hunks {
		on, nn := h.counts()
		os, ns := h.oldStart, h.newStart
		if f.status == "A" {
			os = 0
		}
		sec := ""
		if h.section != "" {
			sec = " " + h.section
		}
		fmt.Fprintf(&b, "@@ -%s +%s @@%s\n", rangeText(os, on), rangeText(ns, nn), sec)
		for _, l := range h.lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func rangeText(start, n int) string {
	if n == 1 {
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d,%d", start, n)
}

// side returns one side of a hunk's lines ('-' old, '+' new), without the
// diff prefix.
func (h hunk) side(newSide bool) []string {
	var out []string
	for _, l := range h.lines {
		switch {
		case l[0] == ' ':
			out = append(out, l[1:])
		case l[0] == '-' && !newSide:
			out = append(out, l[1:])
		case l[0] == '+' && newSide:
			out = append(out, l[1:])
		}
	}
	return out
}

// content rebuilds a file from its hunks, filling the unchanged stretches
// between them with fill(from, to) lines (1-based, inclusive).
func (f fileDiff) content(newSide bool, fill func(from, to int) []string) string {
	var out []string
	for _, h := range f.hunks {
		start := h.newStart
		if !newSide {
			start = h.oldStart
		}
		if f.status == "A" && !newSide {
			continue
		}
		if start > len(out)+1 {
			out = append(out, fill(len(out)+1, start-1)...)
		}
		out = append(out, h.side(newSide)...)
	}
	return strings.Join(out, "\n") + "\n"
}

func lines(s string) []string { return strings.Split(strings.TrimPrefix(s, "\n"), "\n") }

// prefixed turns plain source lines into diff lines with the prefix p.
func prefixed(p string, src ...string) []string {
	out := make([]string, len(src))
	for i, l := range src {
		out[i] = p + l
	}
	return out
}

func join(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
