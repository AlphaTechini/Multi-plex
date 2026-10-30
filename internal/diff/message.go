package diff

import (
	"strconv"
	"strings"
)

// CommentItem is one comment in the message sent to the agent.
type CommentItem struct {
	Path       string // relative to the workspace root, slash-separated
	Start, End int    // new-side lines, 1-based inclusive
	Language   string // fence language name, "" for none
	Snippet    string // the selected lines
	Body       string // the comment text
}

// FormatComments renders queued comments as the single message described in
// docs/07-diff-viewer.md ("Sending to the agent"):
//
//	Review comments (session: auth-refactor):
//
//	1. src/auth/session.go:L12-L30
//	   ```go
//	   <the selected lines>
//	   ```
//	   Comment: this refresh logic runs on every request.
//
// A single line is written path:L88 and a range path:L12-L30. The code fence
// uses more backticks than any run inside the snippet. The result has no
// trailing newline.
func FormatComments(sessionName string, items []CommentItem) string {
	var b strings.Builder
	b.WriteString("Review comments (session: ")
	b.WriteString(sessionName)
	b.WriteString("):")
	for i, it := range items {
		num := strconv.Itoa(i+1) + ". "
		indent := strings.Repeat(" ", len(num))
		b.WriteString("\n\n")
		b.WriteString(num)
		b.WriteString(it.Path)
		b.WriteString(":L")
		b.WriteString(strconv.Itoa(it.Start))
		if it.End > it.Start {
			b.WriteString("-L")
			b.WriteString(strconv.Itoa(it.End))
		}
		fence := strings.Repeat("`", fenceLen(it.Snippet))
		b.WriteString("\n" + indent + fence + it.Language)
		writeIndented(&b, indent, it.Snippet)
		b.WriteString("\n" + indent + fence)
		b.WriteString("\n" + indent + "Comment: ")
		writeBody(&b, indent, it.Body)
	}
	return b.String()
}

// fenceLen returns the backtick count for a fence around s: at least 3, and
// longer than any run of backticks in s.
func fenceLen(s string) int {
	longest, run := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return max(3, longest+1)
}

// writeIndented writes each line of s on its own line, indented.
func writeIndented(b *strings.Builder, indent, s string) {
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		b.WriteByte('\n')
		if l != "" {
			b.WriteString(indent)
			b.WriteString(l)
		}
	}
}

// writeBody writes the comment text; continuation lines are indented.
func writeBody(b *strings.Builder, indent, s string) {
	for i, l := range strings.Split(strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n") {
		if i > 0 {
			b.WriteByte('\n')
			if l != "" {
				b.WriteString(indent)
			}
		}
		b.WriteString(l)
	}
}
