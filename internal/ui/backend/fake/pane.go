package fake

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// fakePane is a scripted terminal: an agent transcript or a shell. Only
// frames are produced from it; there is no real emulator.
type fakePane struct {
	id      core.PaneID
	session core.SessionID
	kind    core.PaneKind
	slug    string
	cols    int
	rows    int
	exited  bool
	code    int

	// agent
	entries  []entry
	working  bool
	startedT time.Time
	spin     int
	now      func() time.Time

	// both
	input    string
	shell    []string // shell scrollback
	cwdLabel string
	lastRows []string
}

type entryKind int

const (
	entTool entryKind = iota
	entSay
	entUser
)

// entry is one transcript item.
type entry struct {
	kind         entryKind
	name, arg    string // tool name and argument
	result, text string // tool result line; say / user text
}

func newAgentPane(s core.Session, now func() time.Time) *fakePane {
	p := &fakePane{id: s.AgentPane, session: s.ID, kind: core.PaneAgent, slug: s.Slug, cols: 56, rows: 46, now: now, startedT: now()}
	p.entries = seedTranscript(s.Slug)
	return p
}

func newShellPane(id core.PaneID, s core.Session, cols, rows int) *fakePane {
	p := &fakePane{id: id, session: s.ID, kind: core.PaneShell, slug: s.Slug, cols: cols, rows: rows}
	p.cwdLabel = "~/dev/multi-plex"
	p.shell = []string{
		sgr("38;5;244", "Multiplex shell · "+s.WorktreePath),
		"",
	}
	return p
}

func (p *fakePane) info() core.Pane {
	return core.Pane{ID: p.id, SessionID: p.session, Kind: p.kind, Cols: p.cols, Rows: p.rows, Exited: p.exited, ExitCode: p.code}
}

// sgr wraps text in an SGR sequence. The colours are the agent CLI's own
// (256-colour indexes), not Multiplex theme tokens.
func sgr(code, text string) string {
	if text == "" {
		return ""
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// fitRow makes a styled line exactly w cells wide.
func fitRow(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if sw := ansi.StringWidth(s); sw > w {
		s = ansi.Truncate(s, w, "")
	} else if sw < w {
		s += strings.Repeat(" ", w-sw)
	}
	return s
}

// render draws the pane's current screen as cols-wide styled rows.
func (p *fakePane) render() []string {
	var lines []string
	cursorRow, cursorCol := -1, 0
	if p.kind == core.PaneShell {
		lines, cursorRow, cursorCol = p.renderShell()
	} else {
		lines, cursorRow, cursorCol = p.renderAgent()
	}
	_ = cursorRow
	_ = cursorCol
	out := make([]string, p.rows)
	for i := range out {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out[i] = fitRow(l, p.cols)
	}
	return out
}

// cursor returns the cursor cell for the current screen.
func (p *fakePane) cursor() (x, y int, visible bool) {
	if p.exited {
		return 0, 0, false
	}
	var row, col int
	if p.kind == core.PaneShell {
		_, row, col = p.renderShell()
	} else {
		_, row, col = p.renderAgent()
	}
	return min(col, p.cols-1), min(max(row, 0), p.rows-1), true
}

func (p *fakePane) renderAgent() (lines []string, cursorRow, cursorCol int) {
	w := p.cols
	for _, e := range p.entries {
		lines = append(lines, "")
		switch e.kind {
		case entTool:
			lines = append(lines, " "+sgr("38;5;114", "●")+" "+sgr("1;38;5;255", e.name)+sgr("38;5;248", "("+e.arg+")"))
			if e.result != "" {
				lines = append(lines, "   "+sgr("38;5;240", "⎿  ")+sgr("38;5;244", e.result))
			}
		case entSay:
			for i, l := range wrapText(e.text, max(w-4, 8)) {
				lead := "   "
				if i == 0 {
					lead = " " + sgr("38;5;252", "●") + " "
				}
				lines = append(lines, lead+sgr("38;5;252", l))
			}
		case entUser:
			for i, l := range wrapText(e.text, max(w-4, 8)) {
				lead := "   "
				if i == 0 {
					lead = " " + sgr("38;5;244", ">") + " "
				}
				lines = append(lines, sgr("48;5;236", fitRow(lead+sgr("38;5;252", l), w)))
			}
		}
	}
	if p.working {
		frames := []string{"✻", "✽", "✶", "✳"}
		secs := int(p.now().Sub(p.startedT).Seconds())
		lines = append(lines, "", " "+sgr("38;5;209", frames[p.spin%len(frames)])+" "+sgr("38;5;209", "Updating callers…")+
			sgr("38;5;240", " ("+itoa(secs)+"s · esc to interrupt)"))
	}
	rule := sgr("38;5;240", strings.Repeat("─", max(w-2, 0)))
	prompt := " " + sgr("38;5;244", ">") + " "
	var inputLine string
	if p.input == "" {
		inputLine = prompt + sgr("38;5;240", `Try "update the callers in api/"`)
	} else {
		inputLine = prompt + sgr("38;5;252", p.input)
	}
	bottom := []string{" " + rule, inputLine, " " + rule, "   " + sgr("38;5;240", "? for shortcuts")}
	body := p.rows - len(bottom)
	if len(lines) > body {
		lines = lines[len(lines)-body:]
	}
	for len(lines) < body {
		lines = append(lines, "")
	}
	cursorRow = len(lines) + 1
	lines = append(lines, bottom...)
	return lines, cursorRow, 3 + utf8.RuneCountInString(p.input)
}

func (p *fakePane) renderShell() (lines []string, cursorRow, cursorCol int) {
	prompt := sgr("1;38;5;114", "dev@multiplex") + sgr("38;5;244", ":") + sgr("1;38;5;75", p.cwdLabel) + sgr("38;5;244", "$ ")
	pw := ansi.StringWidth(prompt)
	all := append(append([]string(nil), p.shell...), prompt+p.input)
	if len(all) > p.rows {
		all = all[len(all)-p.rows:]
	}
	return all, len(all) - 1, pw + utf8.RuneCountInString(p.input)
}

// wrapText word-wraps plain text to w cells.
func wrapText(s string, w int) []string {
	return strings.Split(ansi.Wrap(s, w, ""), "\n")
}

// frame builds a pane.frame payload. A full frame carries every row; a
// partial one only the rows that changed since the last frame, and ok is
// false when nothing changed.
func (p *fakePane) frame(full bool) (protocol.PaneFrame, bool) {
	rows := p.render()
	f := protocol.PaneFrame{PaneID: p.id, Cols: p.cols, Rows: p.rows, Full: full}
	for y, r := range rows {
		if full || y >= len(p.lastRows) || p.lastRows[y] != r {
			f.Changed = append(f.Changed, protocol.Row{Y: y, Text: r})
		}
	}
	f.CursorX, f.CursorY, f.CursorVisible = p.cursor()
	f.Title = p.slug
	if len(f.Changed) == 0 {
		return f, false
	}
	p.lastRows = rows
	return f, true
}

// typeInput feeds bytes from PaneInput into the fake program. It returns
// true when Enter submitted a line, with the text.
func (p *fakePane) typeInput(data []byte) (submitted []string) {
	s := string(data)
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == 0x1b:
			// Skip an escape sequence; a lone Esc interrupts the agent.
			if i+1 >= len(s) {
				if p.kind == core.PaneAgent && p.working {
					p.working = false
					p.entries = append(p.entries, entry{kind: entSay, text: "Interrupted. Tell me what to do differently."})
				}
				i++
				continue
			}
			j := i + 2
			if s[i+1] == '[' || s[i+1] == 'O' {
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				j++
			}
			i = min(j, len(s))
		case c == '\r' || c == '\n':
			if strings.TrimSpace(p.input) != "" || p.kind == core.PaneShell {
				submitted = append(submitted, p.input)
			}
			p.input = ""
			i++
		case c == 0x7f || c == 0x08:
			if r := []rune(p.input); len(r) > 0 {
				p.input = string(r[:len(r)-1])
			}
			i++
		case c == 0x03 || c == 0x15:
			p.input = ""
			i++
		case c < 0x20:
			i++
		default:
			r, n := utf8.DecodeRuneInString(s[i:])
			p.input += string(r)
			i += n
		}
	}
	return submitted
}

// runShell executes a submitted shell line with canned output.
func (p *fakePane) runShell(line string) {
	prompt := sgr("1;38;5;114", "dev@multiplex") + sgr("38;5;244", ":") + sgr("1;38;5;75", p.cwdLabel) + sgr("38;5;244", "$ ")
	p.shell = append(p.shell, prompt+line)
	switch cmd := strings.TrimSpace(line); {
	case cmd == "":
	case cmd == "clear":
		p.shell = nil
	case cmd == "pwd":
		p.shell = append(p.shell, "/home/user/dev/multi-plex")
	case cmd == "ls" || strings.HasPrefix(cmd, "ls "):
		p.shell = append(p.shell, sgr("1;38;5;75", "docs")+"  "+sgr("1;38;5;75", "internal")+"  "+sgr("1;38;5;75", "src")+"  AGENTS.md  CLAUDE.md  README.md  go.mod")
	case cmd == "git status":
		p.shell = append(p.shell,
			"On branch mpx/"+p.slug,
			"Changes not staged for commit:",
			"  "+sgr("38;5;203", "modified:   src/auth/session.go"),
			"  "+sgr("38;5;203", "modified:   src/api/client.go"),
			"Untracked files:",
			"  "+sgr("38;5;203", "src/auth/cache.go"),
		)
	case strings.HasPrefix(cmd, "go test"):
		p.shell = append(p.shell, sgr("38;5;114", "ok")+"  \texample.com/app/internal/auth\t0.412s")
	case strings.HasPrefix(cmd, "echo "):
		p.shell = append(p.shell, strings.TrimPrefix(cmd, "echo "))
	default:
		name, _, _ := strings.Cut(cmd, " ")
		p.shell = append(p.shell, "bash: "+name+": command not found")
	}
	if len(p.shell) > 500 {
		p.shell = p.shell[len(p.shell)-500:]
	}
}

// seedTranscript is each session's transcript at startup.
func seedTranscript(slug string) []entry {
	switch slug {
	case "auth-refactor":
		return []entry{
			{kind: entTool, name: "Read", arg: "src/auth/session.go", result: "Read 182 lines"},
			{kind: entTool, name: "Update", arg: "src/auth/session.go", result: "Updated with 24 additions and 9 removals"},
			{kind: entTool, name: "Bash", arg: "go test ./internal/auth/...", result: "ok   internal/auth   0.412s"},
			{kind: entTool, name: "Update", arg: "src/auth/cache.go", result: "Created with 41 lines"},
			{kind: entSay, text: "Token cache is in place and the auth tests pass. Before I update the callers in api/ and worker/: should the cache be per-user or global? I raised it as card 1."},
		}
	case "api-pagination":
		return []entry{
			{kind: entTool, name: "Read", arg: "src/api/client.go", result: "Read 214 lines"},
			{kind: entTool, name: "Update", arg: "src/api/client.go", result: "Updated with 6 additions and 2 removals"},
			{kind: entTool, name: "Bash", arg: "go test ./internal/api/...", result: "FAIL   internal/api   golden mismatch"},
			{kind: entSay, text: "The golden files encode the old unpaginated shape. I need to delete testdata/golden and regenerate it. Waiting for your approval."},
		}
	default:
		return []entry{
			{kind: entTool, name: "Update", arg: "docs/auth.md", result: "Updated with 12 additions and 1 removal"},
			{kind: entTool, name: "Update", arg: "docs/api.md", result: "Updated with 2 additions and 1 removal"},
			{kind: entSay, text: "Docs are updated for the new auth flow and the pagination parameters. 6 files changed, nothing else to do."},
		}
	}
}
