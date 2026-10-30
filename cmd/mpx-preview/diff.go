package main

import (
	"image/color"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// diffRow is one row of the fake diff: kind, old/new numbers and text.
type diffRow struct {
	kind     kit.DiffKind
	old, new int
	text     string
	folded   int
	selected bool
	comment  bool // render the inline comment box after this row
	fresh    bool // new hunk highlight
}

var sessionDiff = []diffRow{
	{kind: kit.DiffHunk, text: "@@ -38,13 +38,15 @@|func (s *Session) Refresh(ctx context.Context) error"},
	{kind: kit.DiffContext, old: 38, new: 38, text: "func (s *Session) Refresh(ctx context.Context) error {"},
	{kind: kit.DiffContext, old: 39, new: 39, text: "\ts.mu.Lock()"},
	{kind: kit.DiffContext, old: 40, new: 40, text: "\tdefer s.mu.Unlock()"},
	{kind: kit.DiffDel, old: 41, text: "\tif «time.Until(s.expiry) > refreshWindow» {"},
	{kind: kit.DiffAdd, new: 41, text: "\tif «s.tokenValid(time.Now())» {"},
	{kind: kit.DiffContext, old: 42, new: 42, text: "\t\treturn nil"},
	{kind: kit.DiffContext, old: 43, new: 43, text: "\t}"},
	{kind: kit.DiffDel, old: 44, text: "\ttok, err := «s.client.Token(ctx)»", selected: true},
	{kind: kit.DiffAdd, new: 44, text: "\ttok, err := «s.cache.Get(ctx, s.userID, s.client.Token)»", selected: true},
	{kind: kit.DiffContext, old: 45, new: 45, text: "\tif err != nil {", selected: true},
	{kind: kit.DiffDel, old: 46, text: "\t\treturn «err»", selected: true},
	{kind: kit.DiffAdd, new: 46, text: "\t\treturn «fmt.Errorf(\"refresh token: %w\", err)»", selected: true, comment: true},
	{kind: kit.DiffContext, old: 47, new: 47, text: "\t}"},
	{kind: kit.DiffAdd, new: 48, text: "\ts.metrics.Refreshes.Inc()"},
	{kind: kit.DiffContext, old: 48, new: 49, text: "\ts.token, s.expiry = tok.Value, tok.Expiry"},
	{kind: kit.DiffContext, old: 49, new: 50, text: "\treturn nil"},
	{kind: kit.DiffContext, old: 50, new: 51, text: "}"},
	{kind: kit.DiffFold, folded: 42},
	{kind: kit.DiffHunk, text: "@@ -93,6 +95,14 @@|func (s *Session) Close() error", fresh: true},
	{kind: kit.DiffContext, old: 93, new: 95, text: "\treturn s.client.Close()", fresh: true},
	{kind: kit.DiffContext, old: 94, new: 96, text: "}", fresh: true},
	{kind: kit.DiffContext, old: 95, new: 97, text: "", fresh: true},
	{kind: kit.DiffAdd, new: 98, text: "// tokenValid reports whether the token outlives the refresh window.", fresh: true},
	{kind: kit.DiffAdd, new: 99, text: "func (s *Session) tokenValid(now time.Time) bool {", fresh: true},
	{kind: kit.DiffAdd, new: 100, text: "\treturn s.token != \"\" && s.expiry.Sub(now) > refreshWindow", fresh: true},
	{kind: kit.DiffAdd, new: 101, text: "}", fresh: true},
	{kind: kit.DiffContext, old: 96, new: 102, text: "", fresh: true},
}

type changedFile struct {
	dir, name, status string
	add, del          int
	reviewed          bool
}

var changedFiles = []changedFile{
	{"src/api/", "client.go", "M", 6, 2, false},
	{"src/auth/", "cache.go", "A", 41, 0, true},
	{"src/auth/", "session.go", "M", 24, 9, false},
	{"src/auth/", "session_test.go", "M", 88, 4, false},
	{"src/auth/", "token.go", "R", 3, 3, true},
	{"src/worker/", "refresh.go", "M", 5, 5, false},
	{"docs/", "auth.md", "M", 12, 1, false},
}

func (s scene) diff() string {
	return s.screen(screenOpts{mode: 1, focus: focusMain, diffMode: true}, s.diffBody, "unified · 7 files")
}

func (s scene) diffBody(w, h int) string {
	t := s.t
	f := computeLayout(s.w, s.h, true)
	if f.main < fileRailMinFor {
		return s.diffView(w, h, false)
	}
	rail := s.fileRail(fileRailWidth, h)
	sep := strings.TrimSuffix(strings.Repeat(paint(t.Border.Subtle, false, "│")+"\n", h), "\n")
	return kit.JoinColumns(rail, kit.Backfill(sep, t.Surface.Base), s.diffView(w-fileRailWidth-1, h, true))
}

func (s scene) fileRail(w, h int) string {
	t := s.t
	var lines []string
	lines = append(lines, "", kit.Section(t, kit.SectionOptions{Width: w, Label: "Files", Meta: "2/7 reviewed"}), "")
	lastDir := ""
	for i, cf := range changedFiles {
		if cf.dir != lastDir {
			if lastDir != "" {
				lines = append(lines, "")
			}
			lines = append(lines, kit.Fit("  "+paint(t.Text.Muted, false, cf.dir), w))
			lastDir = cf.dir
		}
		check := " "
		if cf.reviewed {
			check = paint(t.State.Done, false, t.Glyphs.Check)
		}
		status := paint(statusColor(s, cf.status), true, cf.status)
		nameC := t.Text.Primary
		if cf.reviewed {
			nameC = t.Text.Muted
		}
		counts := ""
		if cf.add > 0 {
			counts += paint(t.Diff.AddSign, false, "+"+itoa(cf.add))
		}
		if cf.del > 0 {
			counts += " " + paint(t.Diff.DelSign, false, t.Glyphs.Minus+itoa(cf.del))
		}
		lines = append(lines, kit.Row(t, kit.RowOptions{
			Width: w, Selected: i == 2, Focused: false,
			Left:  " " + check + " " + status + "  " + paint(nameC, i == 2, cf.name),
			Right: counts,
		}))
	}
	for len(lines) < h-2 {
		lines = append(lines, "")
	}
	lines = append(lines[:h-2], kit.Fit("  "+kit.KeyHints(t, []kit.Hint{{Keys: "[", Label: "prev"}, {Keys: "]", Label: "next"}, {Keys: "v", Label: "reviewed"}}, 0), w), "")
	return kit.Backfill(kit.FitBlock(strings.Join(lines, "\n"), w, h), t.Surface.Base)
}

func statusColor(s scene, st string) color.Color {
	switch st {
	case "A":
		return s.t.Diff.AddSign
	case "D":
		return s.t.Diff.DelSign
	case "R":
		return s.t.Diff.Renamed
	}
	return s.t.Diff.Modified
}

func (s scene) diffView(w, h int, withRail bool) string {
	t := s.t
	var lines []string
	add := func(block string) { lines = append(lines, strings.Split(block, "\n")...) }
	add(kit.Banner(t, kit.BannerOptions{
		Width: w, Path: "src/auth/session.go", Index: 3, Total: 7,
		Status: "modified", Added: 24, Removed: 9, Settle: s.settle,
	}))
	if !withRail {
		add(kit.Fit("  "+kit.KeyHints(t, []kit.Hint{{Keys: "f", Label: "files"}, {Keys: "[", Label: "prev file"}, {Keys: "]", Label: "next file"}, {Keys: "n", Label: "next change"}, {Keys: "c", Label: "comment"}}, 0), w))
	} else {
		lines = append(lines, "")
	}
	highlight := s.fresh
	for _, r := range sessionDiff {
		o := kit.DiffLineOptions{Width: w, Kind: r.kind, Old: r.old, New: r.new, Selected: r.selected, Folded: r.folded}
		if r.fresh {
			o.Highlight = highlight
		}
		switch r.kind {
		case kit.DiffHunk:
			head, ctx, _ := strings.Cut(r.text, "|")
			o.Header = head
			o.Spans = goSpans(t, ctx)
			for i := range o.Spans {
				o.Spans[i].Fg = t.Text.Muted
			}
		case kit.DiffFold:
		default:
			o.Spans = goSpans(t, r.text)
		}
		add(kit.DiffLine(t, o))
		if r.comment {
			add(kit.CommentBox(t, kit.CommentBoxOptions{
				Width: w, Indent: kit.DiffGutterWidth(4) - 2,
				Author: "You", Status: "draft", Range: "L44" + "–" + "L46",
				Body:    "Wrap with the user ID too, so the logs say whose refresh failed. And this path runs on every request: keep the cache lookup lock-free.",
				Hints:   []kit.Hint{{Keys: "enter", Label: "queue"}, {Keys: "ctrl+s", Label: "send now"}, {Keys: "esc", Label: "cancel"}},
				Focused: true,
			}))
		}
	}
	lines = append(lines, "")
	add(kit.FileEnd(t, kit.FileEndOptions{Width: w, Next: "src/auth/session_test.go", Keys: "]", Total: 7}))
	body := kit.Backfill(kit.FitBlock(strings.Join(lines, "\n"), w, h), t.Surface.Base)
	if s.fade < 1 && t.Animated() {
		// Content fades in under the banner after a file switch.
		bl := kit.Lines(body)
		rest := kit.Dim(t, strings.Join(bl[1:], "\n"), t.Surface.Base, 1-easeOut(s.fade))
		body = bl[0] + "\n" + rest
	}
	toast := kit.Toast(t, kit.ToastOptions{
		Width: 46, Tone: kit.ToneSuccess,
		Title: "Comment queued for auth-refactor",
		Body:  "2 queued · sent at the agent's next turn",
		Hints: []kit.Hint{{Keys: "ctrl+s", Label: "send now"}, {Keys: "u", Label: "undo"}},
	})
	if h < 40 {
		return body
	}
	return kit.Composite(body, toast, w-kit.Width(toast)-2, h-len(kit.Lines(toast))-1)
}
