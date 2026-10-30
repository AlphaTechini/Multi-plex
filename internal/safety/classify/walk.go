package classify

import (
	"strings"
)

// Simple is one simple command after parsing: wrappers (sudo, env, xargs,
// bash -c ...) are looked through, environment assignments removed, and
// redirections split out. The gate uses it to find paths in commands.
type Simple struct {
	Argv      []string   // command word first; git global options removed
	Env       []string   // leading NAME=value assignments
	Redirects []Redirect // output and input redirections
	Cwd       string     // effective working directory ("" when unknown)
	CwdKnown  bool
	PrevArgv  []string // previous command of a pipeline, or find for -exec
	Piped     bool     // fed by a pipe
	ViaXargs  bool     // arguments also come from standard input / find
	Fold      bool     // flags are case-insensitive (PowerShell)
	text      string   // words joined plus redirections, for regex rules
	fl        flavour
}

// Name is the normalised command name: base name, lower-case, without a
// Windows executable extension.
func (s Simple) Name() string { return cmdName(s.Argv[0]) }

// Operands returns the path-like arguments of the command: everything that
// is not a flag (for find, only its start paths). Quotes are already removed.
func (s Simple) Operands() []string { return operandsOf(&s, s.Name(), s.fl) }

// CommandName normalises a command word the way rules do.
func CommandName(word string) string { return cmdName(word) }

const (
	maxDepth    = 10
	maxSegments = 4096
	maxLen      = 1 << 20
)

// wstate is the walker's running state: the working directory and the
// previous command, updated as segments are visited in order.
type wstate struct {
	cwd   string
	known bool
	prev  []string
}

type walker struct {
	cx          cctx
	visitText   func(text string, st *wstate) bool
	visit       func(s *Simple) bool
	stop        bool
	segments    int
	tooComplex  bool
	unparseable bool
}

func newWalker(ctx Context) *walker {
	return &walker{cx: newCctx(ctx)}
}

// run walks a complete command line.
func (w *walker) run(cmd string) {
	if len(cmd) > maxLen {
		w.tooComplex = true
		return
	}
	st := &wstate{cwd: w.cx.Cwd, known: w.cx.Cwd != ""}
	w.text(cmd, w.cx.fl, st, 0)
}

func (w *walker) text(src string, fl flavour, st *wstate, depth int) {
	if w.stop {
		return
	}
	if depth > maxDepth {
		w.tooComplex = true
		return
	}
	if w.visitText != nil && w.visitText(src, st) {
		w.stop = true
		return
	}
	var prev []string
	for _, seg := range splitCommands(src, fl) {
		if w.stop {
			return
		}
		w.segments++
		if w.segments > maxSegments {
			w.tooComplex = true
			w.stop = true
			return
		}
		if !seg.piped {
			prev = nil
		}
		argv := w.segment(seg, fl, st, depth, prev)
		prev = argv
	}
}

var reservedWords = setOf("if", "then", "else", "elif", "do", "while", "until", "!", "{", "}", "time", "coproc")
var closers = setOf("fi", "done", "esac", "}")
var psKeywords = setOf("if", "elseif", "else", "foreach", "for", "while", "do", "until", "switch", "try", "catch",
	"finally", "trap", "function", "filter", "param", "begin", "process", "end", "return", "break", "continue")

// segment visits one simple command (and everything nested in it) and
// returns its final argv so the next pipeline stage can see it.
func (w *walker) segment(seg rawSegment, fl flavour, st *wstate, depth int, prev []string) []string {
	words := seg.words
	if len(words) >= 4 && words[0] == "for" && words[2] == "in" {
		items := words[3:]
		for i, it := range items {
			if it == "do" {
				items = items[:i]
				break
			}
		}
		if w.cx.loopVars == nil {
			w.cx.loopVars = map[string][]string{}
		}
		w.cx.loopVars[words[1]] = items
	}
	words, drop := stripLeading(words, fl)
	var env []string
	if !drop {
		words, env = stripAssignments(words, fl)
	}
	w.subs(seg, fl, st, depth)
	w.noteTempVars(env, len(words) == 0)
	if drop || len(words) == 0 {
		return nil
	}

	viaXargs := false
	var xargsPrev []string
	for i := 0; i < 8 && len(words) > 0; i++ {
		name := cmdName(words[0])
		u, ok := unwrap(name, words, fl)
		if !ok {
			break
		}
		if u.isTxt {
			w.text(u.text, u.fl, st.copy(), depth+1)
			return nil
		}
		if name == "xargs" {
			viaXargs = true
			xargsPrev = prev
		}
		words = u.rest
	}
	if len(words) == 0 {
		return nil
	}
	name := cmdName(words[0])

	// A shell reading its script from a here-document or a pipe.
	if shellNames[name] || psNames[name] {
		if seg.heredoc != "" {
			w.text(seg.heredoc, flavourFor(name), st.copy(), depth+1)
		} else if seg.piped && len(prev) > 0 && !hasScriptArg(words[1:]) {
			switch cmdName(prev[0]) {
			case "echo", "printf", "write-output", "write-host", "cat", "type":
				w.text(strings.Join(prev[1:], " "), flavourFor(name), st.copy(), depth+1)
			}
		}
	}
	for _, code := range embeddedCommands(name, words[1:]) {
		w.text(code, flBash, st.copy(), depth+1)
	}

	s := &Simple{
		Argv: words, Env: env, Redirects: seg.redirs, Cwd: st.cwd, CwdKnown: st.known,
		Piped: seg.piped, ViaXargs: viaXargs, fl: fl,
		Fold: fl == flPowerShell || isCmdlet(name) || psAlias[name],
	}
	if seg.piped {
		s.PrevArgv = prev
	}
	if viaXargs {
		s.PrevArgv = xargsPrev
	}
	s.text = segmentText(words, seg.redirs)
	if name == "git" {
		argv, dir := normalizeGit(words)
		s.Argv = argv
		if dir != "" {
			s.Cwd, s.CwdKnown = w.cx.resolve(dir, st.cwd, st.known)
		}
	}
	if w.visit != nil && w.visit(s) {
		w.stop = true
		return words
	}
	if name == "find" {
		w.findExec(s, st, depth)
	}
	w.trackCwd(s, name, st)
	return words
}

var psAlias = setOf("rm", "ri", "rd", "del", "erase", "rmdir", "gci", "gc", "sl", "cp", "mv", "mi", "ni", "sc", "ac")

// noteTempVars remembers variables assigned from mktemp ("tmp=$(mktemp -d)")
// so "rm -rf $tmp" is recognised as a delete inside the temp directory. Only
// plain assignment statements count.
func (w *walker) noteTempVars(env []string, bare bool) {
	if !bare {
		return
	}
	for _, e := range env {
		name, val, _ := strings.Cut(e, "=")
		if strings.HasPrefix(val, "$(mktemp") || strings.HasPrefix(val, "`mktemp") {
			if w.cx.tmpVars == nil {
				w.cx.tmpVars = map[string]bool{}
			}
			w.cx.tmpVars[name] = true
		}
	}
}

func flavourFor(name string) flavour {
	if psNames[name] {
		return flPowerShell
	}
	return flBash
}

func (s *wstate) copy() *wstate { c := *s; c.prev = nil; return &c }

// subs walks command substitutions and groups found in a segment's words.
func (w *walker) subs(seg rawSegment, fl flavour, st *wstate, depth int) {
	for _, sub := range seg.subs {
		if w.stop {
			return
		}
		w.text(sub, fl, st.copy(), depth+1)
	}
}

func segmentText(words []string, redirs []Redirect) string {
	t := strings.Join(words, " ")
	for _, r := range redirs {
		t += " " + r.Op + " " + r.Target
	}
	return t
}

// stripLeading removes shell reserved words and loop headers. drop is true
// when the whole segment is syntax with no command.
func stripLeading(words []string, fl flavour) (rest []string, drop bool) {
	for len(words) > 0 {
		w0 := words[0]
		lw := strings.ToLower(w0)
		switch {
		case fl == flPowerShell && psKeywords[lw]:
			return nil, true
		case closers[w0] && len(words) == 1:
			return nil, true
		case w0 == "for" || w0 == "select":
			for i, x := range words {
				if x == "do" {
					words = words[i+1:]
					goto next
				}
			}
			return nil, true
		case w0 == "function" || w0 == "case":
			return nil, true
		case fl == flCmd && lw == "if":
			words = stripCmdIf(words[1:])
		case reservedWords[w0]:
			words = words[1:]
		default:
			return words, false
		}
	next:
	}
	return nil, true
}

// stripCmdIf skips the condition of a cmd.exe "if" statement.
func stripCmdIf(w []string) []string {
	if len(w) > 0 && strings.EqualFold(w[0], "/i") {
		w = w[1:]
	}
	if len(w) > 0 && strings.EqualFold(w[0], "not") {
		w = w[1:]
	}
	if len(w) == 0 {
		return w
	}
	switch strings.ToLower(w[0]) {
	case "exist", "defined", "errorlevel", "cmdextversion":
		if len(w) >= 2 {
			return w[2:]
		}
		return nil
	}
	if strings.Contains(w[0], "==") {
		return w[1:]
	}
	if len(w) >= 3 {
		return w[3:] // a EQU b
	}
	return nil
}

// stripAssignments removes leading NAME=value words (and PowerShell variable
// assignments), returning the POSIX-style ones.
func stripAssignments(words []string, fl flavour) (rest, env []string) {
	for len(words) > 0 {
		w0 := words[0]
		switch {
		case isAssignment(w0):
			env = append(env, w0)
			words = words[1:]
		case fl == flPowerShell && strings.HasPrefix(w0, "$"):
			if len(words) >= 2 && (words[1] == "=" || words[1] == "+=") {
				words = words[2:]
			} else if strings.Contains(w0, "=") {
				words = words[1:]
			} else {
				return words, env
			}
		default:
			return words, env
		}
	}
	return nil, env
}

var cdNames = setOf("cd", "chdir", "pushd", "set-location", "sl")

// trackCwd follows cd so later relative paths resolve against the new
// directory.
func (w *walker) trackCwd(s *Simple, name string, st *wstate) {
	if !cdNames[name] {
		return
	}
	var target string
	for _, a := range s.Argv[1:] {
		if len(a) > 1 && a[0] == '-' || (w.cx.fl == flCmd && isSlashFlag(a)) {
			if a == "-" {
				break
			}
			continue
		}
		target = a
		break
	}
	if target == "" {
		st.cwd, st.known = w.cx.home, w.cx.home != ""
		return
	}
	st.cwd, st.known = w.cx.resolve(target, s.Cwd, s.CwdKnown)
}

// findExec visits the command run by find -exec/-execdir/-ok as if it were
// fed by find, so its operands include find's start paths.
func (w *walker) findExec(s *Simple, st *wstate, depth int) {
	args := s.Argv[1:]
	for i := 0; i < len(args) && !w.stop; i++ {
		switch args[i] {
		case "-exec", "-execdir", "-ok", "-okdir":
			j := i + 1
			for j < len(args) && args[j] != ";" && args[j] != "+" {
				j++
			}
			inner := args[i+1 : j]
			if len(inner) > 0 {
				w.simpleFrom(inner, s, st, depth)
			}
			i = j
		}
	}
}

// simpleFrom classifies a command that find runs.
func (w *walker) simpleFrom(inner []string, parent *Simple, st *wstate, depth int) {
	words := inner
	for i := 0; i < 8 && len(words) > 0; i++ {
		u, ok := unwrap(cmdName(words[0]), words, w.cx.fl)
		if !ok {
			break
		}
		if u.isTxt {
			w.text(u.text, u.fl, st.copy(), depth+1)
			return
		}
		words = u.rest
	}
	if len(words) == 0 {
		return
	}
	c := &Simple{Argv: words, Cwd: parent.Cwd, CwdKnown: parent.CwdKnown, ViaXargs: true, PrevArgv: parent.Argv,
		Fold: parent.Fold, text: segmentText(words, nil), fl: parent.fl}
	if w.visit != nil && w.visit(c) {
		w.stop = true
	}
}
