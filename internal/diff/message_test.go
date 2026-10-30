package diff

import (
	"strings"
	"testing"
)

func TestFormatComments(t *testing.T) {
	got := FormatComments("auth-refactor", []CommentItem{
		{Path: "src/auth/session.go", Start: 12, End: 30, Language: "go", Snippet: "a := 1\n\nb := 2", Body: "cache the token instead."},
		{Path: "src/auth/session_test.go", Start: 88, End: 88, Language: "go", Snippet: "x()", Body: "line one\nline two"},
	})
	want := "Review comments (session: auth-refactor):\n" +
		"\n" +
		"1. src/auth/session.go:L12-L30\n" +
		"   ```go\n" +
		"   a := 1\n" +
		"\n" +
		"   b := 2\n" +
		"   ```\n" +
		"   Comment: cache the token instead.\n" +
		"\n" +
		"2. src/auth/session_test.go:L88\n" +
		"   ```go\n" +
		"   x()\n" +
		"   ```\n" +
		"   Comment: line one\n" +
		"   line two"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatCommentsFence(t *testing.T) {
	got := FormatComments("s", []CommentItem{{Path: "README.md", Start: 1, End: 3, Language: "markdown",
		Snippet: "```go\nx\n```\n````", Body: "b"}})
	if !strings.Contains(got, "   `````markdown\n") || !strings.Contains(got, "\n   `````\n") {
		t.Errorf("fence not widened:\n%s", got)
	}
}

func TestFormatCommentsEdgeCases(t *testing.T) {
	if got := FormatComments("s", nil); got != "Review comments (session: s):" {
		t.Errorf("empty = %q", got)
	}
	got := FormatComments("s", []CommentItem{{Path: "a", Start: 5, End: 2, Snippet: "x", Body: "b\n"}})
	if !strings.Contains(got, "1. a:L5\n") || !strings.HasSuffix(got, "Comment: b") {
		t.Errorf("got %q", got)
	}
	// No language: bare fence. Double-digit numbering keeps indentation.
	items := make([]CommentItem, 10)
	for i := range items {
		items[i] = CommentItem{Path: "p", Start: 1, End: 1, Snippet: "s", Body: "b"}
	}
	got = FormatComments("s", items)
	if !strings.Contains(got, "\n\n10. p:L1\n    ```\n    s\n    ```\n    Comment: b") {
		t.Errorf("tenth item:\n%s", got)
	}
}
