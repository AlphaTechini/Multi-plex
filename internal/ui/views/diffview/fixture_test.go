package diffview

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// fakeBackend implements backend.Backend by embedding the interface (every
// method this view does not use panics on a nil receiver) and overriding the
// ones it calls.
type fakeBackend struct {
	backend.Backend

	mu       sync.Mutex
	raw      string
	changes  []vcs.FileChange
	files    map[string]string
	comments []core.Comment
	nextID   int
	diffs    int
	reads    map[string]int
	sent     int
	sendMsg  string
	diffErr  error
}

func newFake() *fakeBackend {
	return &fakeBackend{files: map[string]string{}, reads: map[string]int{}}
}

func (f *fakeBackend) Diff(_ context.Context, _ core.SessionID) (protocol.DiffResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.diffs++
	if f.diffErr != nil {
		return protocol.DiffResult{}, f.diffErr
	}
	return protocol.DiffResult{Base: "base1", Head: "head1", Raw: f.raw, Changes: f.changes}, nil
}

func (f *fakeBackend) ReadFile(_ context.Context, _ core.SessionID, path, _ string) (protocol.FileReadResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads[path]++
	c, ok := f.files[path]
	if !ok {
		return protocol.FileReadResult{}, fmt.Errorf("no such file %s", path)
	}
	return protocol.FileReadResult{Path: path, Content: c}, nil
}

func (f *fakeBackend) Comments(_ context.Context, _ core.SessionID) ([]core.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.Comment(nil), f.comments...), nil
}

func (f *fakeBackend) PutComment(_ context.Context, c core.Comment) (core.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c.ID == "" {
		f.nextID++
		c.ID = "c" + strconv.Itoa(f.nextID)
	}
	for i := range f.comments {
		if f.comments[i].ID == c.ID {
			f.comments[i] = c
			return c, nil
		}
	}
	f.comments = append(f.comments, c)
	return c, nil
}

func (f *fakeBackend) DeleteComment(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.comments {
		if f.comments[i].ID == id {
			f.comments = append(f.comments[:i], f.comments[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeBackend) SendComments(_ context.Context, _ core.SessionID) (protocol.CommentSendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for i := range f.comments {
		if f.comments[i].Status == core.CommentQueued {
			f.comments[i].Status = core.CommentSent
			n++
		}
	}
	f.sent += n
	return protocol.CommentSendResult{Sent: n, Message: "Review comments"}, nil
}

func (f *fakeBackend) snapshotComments() []core.Comment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.Comment(nil), f.comments...)
}

// fakeClock is a settable clock.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// harness drives a Model like the shell would: it delivers messages and runs
// the returned commands synchronously, feeding their results back.
type harness struct {
	t      *testing.T
	m      *Model
	be     *fakeBackend
	clock  *fakeClock
	toasts []view.ToastMsg
	clip   []string
	// skipTick makes run skip timer commands (debounce) instead of waiting.
	skipTick bool
	pending  []tea.Cmd
}

var testSession = &core.Session{ID: "s1", Slug: "auth-refactor", WorktreePath: "/wt"}

// newHarness builds a view on the fixture diff, focused, with the session
// selected and the first diff loaded.
func newHarness(t *testing.T, th *theme.Theme, be *fakeBackend) *harness {
	t.Helper()
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	env := view.Env{Theme: th, Backend: be}
	m := newModel(env, testHL{}, clock.now)
	h := &harness{t: t, m: m, be: be, clock: clock, skipTick: true}
	h.send(view.FocusMsg{Focused: true})
	h.send(view.SessionMsg{Session: testSession})
	return h
}

// send delivers a message and runs the commands it produces.
func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	h.run(cmd)
}

// run executes a command and feeds its message back to the model.
func (h *harness) run(cmd tea.Cmd) {
	h.t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	switch msg := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			h.run(c)
		}
	case view.ToastMsg:
		h.toasts = append(h.toasts, msg)
	case refreshDueMsg:
		h.send(msg)
	default:
		if v := reflect.ValueOf(msg); v.Kind() == reflect.String && strings.Contains(reflect.TypeOf(msg).Name(), "lipboard") {
			h.clip = append(h.clip, v.String())
			return
		}
		h.send(msg)
	}
}

// key sends a key press by its name ("j", "ctrl+d", "enter").
func (h *harness) key(names ...string) {
	h.t.Helper()
	for _, n := range names {
		h.send(keyMsg(n))
	}
}

// keyMsg builds a key press from a name.
func keyMsg(name string) tea.KeyPressMsg {
	special := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "down": tea.KeyDown, "up": tea.KeyUp,
		"left": tea.KeyLeft, "right": tea.KeyRight, "pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
		"home": tea.KeyHome, "end": tea.KeyEnd, "backspace": tea.KeyBackspace, "tab": tea.KeyTab,
	}
	if r, ok := special[name]; ok {
		return tea.KeyPressMsg{Code: r}
	}
	if strings.HasPrefix(name, "ctrl+") {
		return tea.KeyPressMsg{Code: rune(name[5]), Mod: tea.ModCtrl}
	}
	if strings.HasPrefix(name, "shift+") {
		n := name[6:]
		if r, ok := special[n]; ok {
			return tea.KeyPressMsg{Code: r, Mod: tea.ModShift}
		}
		r := []rune(n)[0]
		return tea.KeyPressMsg{Code: r, Mod: tea.ModShift, Text: strings.ToUpper(n)}
	}
	r := []rune(name)
	if len(r) == 1 {
		return tea.KeyPressMsg{Code: r[0], Text: name}
	}
	panic("keyMsg: unknown key " + name)
}

// typeText sends text as key presses.
func (h *harness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// render returns the rendered view at w × hh.
func (h *harness) render(w, hh int) string { return h.m.Render(w, hh) }

// current returns the path of the current file.
func (h *harness) current() string {
	if h.m.fv == nil {
		return ""
	}
	return h.m.fv.e.path
}

// ---- fixture diff: the docs/16 mockup ----

// hunkText builds a hunk from lines prefixed with ' ', '+' or '-'.
func hunkText(oldStart, newStart int, header string, lines ...string) string {
	var o, n int
	for _, l := range lines {
		switch l[0] {
		case ' ':
			o++
			n++
		case '-':
			o++
		case '+':
			n++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@", oldStart, o, newStart, n)
	if header != "" {
		b.WriteString(" " + header)
	}
	b.WriteString("\n")
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	return b.String()
}

// fileDiff wraps hunks in a git file header.
func fileDiff(path string, hunks ...string) string {
	return "diff --git a/" + path + " b/" + path + "\nindex 1111111..2222222 100644\n--- a/" + path + "\n+++ b/" + path + "\n" + strings.Join(hunks, "")
}

func newFileDiff(path string, lines ...string) string {
	var b strings.Builder
	b.WriteString("diff --git a/" + path + " b/" + path + "\nnew file mode 100644\nindex 0000000..2222222\n--- /dev/null\n+++ b/" + path + "\n")
	fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, l := range lines {
		b.WriteString("+" + l + "\n")
	}
	return b.String()
}

// sessionGoHunks are the two hunks of src/auth/session.go from the mockup.
func sessionGoHunks() []string {
	h1 := hunkText(38, 38, "func (s *Session) Refresh(ctx context.Context) error",
		" func (s *Session) Refresh(ctx context.Context) error {",
		" \ts.mu.Lock()",
		" \tdefer s.mu.Unlock()",
		"-\tif time.Until(s.expiry) > refreshWindow {",
		"+\tif s.tokenValid(time.Now()) {",
		" \t\treturn nil",
		" \t}",
		"-\ttok, err := s.client.Token(ctx)",
		"+\ttok, err := s.cache.Get(ctx, s.userID, s.client.Token)",
		" \tif err != nil {",
		"-\t\treturn err",
		"+\t\treturn fmt.Errorf(\"refresh token: %w\", err)",
		" \t}",
		"+\ts.metrics.Refreshes.Inc()",
		" \ts.token, s.expiry = tok.Value, tok.Expiry",
		" \treturn nil",
		" }",
	)
	h2 := hunkText(93, 94, "func (s *Session) Close() error",
		" \treturn s.client.Close()",
		" }",
		" ",
		"+// tokenValid reports whether the token outlives the refresh window.",
		"+func (s *Session) tokenValid(now time.Time) bool {",
		"+\treturn s.token != \"\" && s.expiry.Sub(now) > refreshWindow",
		"+}",
		" ",
	)
	return []string{h1, h2}
}

// sessionGoContent is the new content of src/auth/session.go: the hunks'
// new-side lines at their positions, numbered filler elsewhere.
func sessionGoContent(total int) string {
	lines := make([]string, total)
	for i := range lines {
		lines[i] = fmt.Sprintf("// filler line %d", i+1)
	}
	put := func(start int, texts ...string) {
		for i, t := range texts {
			lines[start-1+i] = t
		}
	}
	put(38,
		"func (s *Session) Refresh(ctx context.Context) error {",
		"\ts.mu.Lock()",
		"\tdefer s.mu.Unlock()",
		"\tif s.tokenValid(time.Now()) {",
		"\t\treturn nil",
		"\t}",
		"\ttok, err := s.cache.Get(ctx, s.userID, s.client.Token)",
		"\tif err != nil {",
		"\t\treturn fmt.Errorf(\"refresh token: %w\", err)",
		"\t}",
		"\ts.metrics.Refreshes.Inc()",
		"\ts.token, s.expiry = tok.Value, tok.Expiry",
		"\treturn nil",
		"}",
	)
	put(94,
		"\treturn s.client.Close()",
		"}",
		"",
		"// tokenValid reports whether the token outlives the refresh window.",
		"func (s *Session) tokenValid(now time.Time) bool {",
		"\treturn s.token != \"\" && s.expiry.Sub(now) > refreshWindow",
		"}",
		"",
	)
	return strings.Join(lines, "\n") + "\n"
}

// mockupFake builds the seven-file session of the docs/16 mockups.
func mockupFake() *fakeBackend {
	be := newFake()
	var raw strings.Builder
	raw.WriteString(fileDiff("src/api/client.go",
		hunkText(10, 10, "func (c *Client) Do(req *Request) (*Response, error)",
			" \tctx := req.Context()",
			"-\tresp, err := c.http.Do(req)",
			"+\tresp, err := c.http.Do(req.WithContext(ctx))",
			" \tif err != nil {",
			"-\t\treturn nil, err",
			"+\t\treturn nil, fmt.Errorf(\"client do: %w\", err)",
			" \t}",
			" \treturn resp, nil",
		)))
	var cache []string
	cache = append(cache, "package auth", "", "// Cache holds refreshed tokens per user.", "type Cache struct {", "\tmu    sync.Mutex", "\ttoks  map[string]Token", "}", "",
		"// Get returns the cached token or fetches a new one.", "func (c *Cache) Get(ctx context.Context, user string, fetch func(context.Context) (Token, error)) (Token, error) {",
		"\tc.mu.Lock()", "\tdefer c.mu.Unlock()", "\tif t, ok := c.toks[user]; ok && t.Valid() {", "\t\treturn t, nil", "\t}", "\tt, err := fetch(ctx)", "\tif err != nil {", "\t\treturn Token{}, err", "\t}", "\tc.toks[user] = t", "\treturn t, nil", "}")
	raw.WriteString(newFileDiff("src/auth/cache.go", cache...))
	hs := sessionGoHunks()
	raw.WriteString(fileDiff("src/auth/session.go", hs...))
	raw.WriteString(fileDiff("src/auth/session_test.go",
		hunkText(80, 80, "func TestRefresh(t *testing.T)",
			" \ts := newSession(t)",
			"-\tif err := s.Refresh(ctx); err != nil {",
			"+\tif err := s.Refresh(context.Background()); err != nil {",
			" \t\tt.Fatal(err)",
			" \t}",
		)))
	raw.WriteString("diff --git a/src/auth/tokens.go b/src/auth/token.go\nsimilarity index 92%\nrename from src/auth/tokens.go\nrename to src/auth/token.go\nindex 3333333..4444444 100644\n--- a/src/auth/tokens.go\n+++ b/src/auth/token.go\n")
	raw.WriteString(hunkText(5, 5, "type Token struct",
		" \tValue  string",
		"-\tExpiry int64",
		"+\tExpiry time.Time",
		" }",
	))
	raw.WriteString(fileDiff("src/worker/refresh.go",
		hunkText(20, 20, "func (w *Worker) refreshAll()",
			" \tfor _, s := range w.sessions {",
			"-\t\tgo s.Refresh(w.ctx)",
			"+\t\tgo func(s *Session) { _ = s.Refresh(w.ctx) }(s)",
			" \t}",
		)))
	raw.WriteString(fileDiff("docs/auth.md",
		hunkText(3, 3, "",
			" Tokens refresh before they expire.",
			"-The refresh window is fixed.",
			"+The refresh window is configurable per environment.",
			" ",
		)))
	be.raw = raw.String()
	be.files["src/auth/session.go"] = sessionGoContent(140)
	for _, p := range []string{"src/api/client.go", "src/auth/cache.go", "src/auth/session_test.go", "src/auth/token.go", "src/worker/refresh.go", "docs/auth.md"} {
		be.files[p] = "// content of " + p + "\n"
	}
	return be
}

// testHL is a tiny highlighter for tests: it marks Go keywords so rendering
// and caching paths run without chroma.
type testHL struct{}

func (testHL) Language(path string) string {
	if strings.HasSuffix(path, ".go") {
		return "go"
	}
	return ""
}

func (testHL) Lines(path, src string) [][]highlight.Token {
	if src == "" {
		return nil
	}
	var out [][]highlight.Token
	for _, line := range strings.Split(strings.TrimSuffix(src, "\n"), "\n") {
		var toks []highlight.Token
		for _, w := range splitWords(line) {
			kind := highlight.Plain
			switch w {
			case "func", "var", "type", "struct", "package":
				kind = highlight.Keyword
			case "if", "return", "for", "defer":
				kind = highlight.Keyword
			case "nil":
				kind = highlight.Constant
			}
			if strings.HasPrefix(w, "//") {
				kind = highlight.Comment
			}
			toks = append(toks, highlight.Token{Text: w, Kind: kind})
		}
		out = append(out, toks)
	}
	return out
}

// splitWords splits a line into words and the separators between them, so
// that concatenating the pieces gives the line.
func splitWords(s string) []string {
	var out []string
	start := 0
	isW := func(c byte) bool {
		return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	for i := 1; i <= len(s); i++ {
		if i == len(s) || isW(s[i]) != isW(s[i-1]) || !isW(s[i]) {
			out = append(out, s[start:i])
			start = i
		}
	}
	if len(s) == 0 {
		return nil
	}
	return out
}

// refresh delivers a diff.changed event and the end of the debounce window.
func (h *harness) refresh() {
	h.t.Helper()
	_, _ = h.m.Update(view.EventMsg{Topic: protocol.TopicDiffChanged})
	h.send(refreshDueMsg{gen: h.m.gen})
}

// wheel builds a mouse wheel message at x, y.
func wheel(x, y int, down bool) tea.MouseWheelMsg {
	b := tea.MouseWheelUp
	if down {
		b = tea.MouseWheelDown
	}
	return tea.MouseWheelMsg{X: x, Y: y, Button: b}
}

// click builds a left click at x, y.
func click(x, y int, mod tea.KeyMod) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: mod}
}
