package classify

import "strings"

// flavour selects the quoting and separator rules of the shell being parsed.
type flavour int

const (
	flBash flavour = iota // POSIX shells; also the default when the shell is unknown
	flPowerShell
	flCmd
)

func flavourOf(shell string) flavour {
	switch strings.ToLower(shell) {
	case "powershell", "pwsh":
		return flPowerShell
	case "cmd":
		return flCmd
	}
	return flBash
}

// Redirect is an input or output redirection of a simple command.
type Redirect struct {
	Op     string // ">", ">>", ">|", ">&", "&>", "&>>", "<", "<&"
	Target string
}

// IsOutput reports whether the redirect writes to its target.
func (r Redirect) IsOutput() bool { return strings.Contains(r.Op, ">") }

// IsSink reports whether the target is a device that discards or shows output
// ("/dev/null", "NUL", "$null", ...), which is never a file write.
func (r Redirect) IsSink() bool {
	t := strings.ToLower(r.Target)
	switch t {
	case "/dev/null", "/dev/stdout", "/dev/stderr", "/dev/tty", "/dev/zero", "nul", "nul:", "$null", "con", "&1", "&2", "-":
		return true
	}
	return strings.HasPrefix(t, "/dev/fd/")
}

// rawSegment is one simple command as written: words with quotes removed,
// redirections split out, command substitutions found inside the words.
type rawSegment struct {
	words     []string
	redirs    []Redirect
	subs      []string // texts of $(...), `...`, <(...) and (...) groups
	heredoc   string   // body of a here-document attached to the command
	hereDelim string
	hereStrip bool
	piped     bool // preceded by a pipe
}

// scanner is a quote-aware splitter for compound commands. It is not a full
// shell parser: it knows enough to split on separators, remove quoting, find
// nested commands and keep redirections out of the argument list.
type scanner struct {
	src      string
	i        int
	fl       flavour
	segs     []rawSegment
	seg      rawSegment
	cur      []byte
	inWord   bool
	pend     string // redirect operator waiting for its target word
	pendHere []int  // indexes of segments waiting for here-document bodies
	piped    bool   // the next segment follows a pipe
}

// splitCommands splits src into simple commands.
func splitCommands(src string, fl flavour) []rawSegment {
	s := &scanner{src: src, fl: fl}
	s.run()
	return s.segs
}

func (s *scanner) run() {
	n := len(s.src)
	for s.i < n {
		c := s.src[s.i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			s.endWord()
			s.i++
		case c == '\n':
			s.endWord()
			s.pend = ""
			keep := s.piped && len(s.seg.words) == 0 && len(s.seg.redirs) == 0 // "a |\n b" continues the pipeline
			s.endSegment(false)
			s.piped = keep
			s.i++
			s.consumeHeredocs()
		case c == '#' && !s.inWord && s.fl != flCmd:
			for s.i < n && s.src[s.i] != '\n' {
				s.i++
			}
		case c == '<' && s.fl == flPowerShell && s.i+1 < n && s.src[s.i+1] == '#':
			end := strings.Index(s.src[s.i:], "#>")
			if end < 0 {
				s.i = n
			} else {
				s.i += end + 2
			}
		case c == ';':
			s.endWord()
			s.endSegment(false)
			s.i++
		case c == '&':
			s.ampersand()
		case c == '|':
			s.endWord()
			switch {
			case s.i+1 < n && s.src[s.i+1] == '|':
				s.endSegment(false)
				s.i += 2
			case s.i+1 < n && s.src[s.i+1] == '&' && s.fl == flBash:
				s.endSegment(true)
				s.i += 2
			default:
				s.endSegment(true)
				s.i++
			}
		case c == '>' || c == '<':
			s.redirect()
		case c == '(' && s.fl == flPowerShell:
			s.group(s.i)
		case c == '(' || c == ')':
			s.endWord()
			s.endSegment(false)
			s.i++
		case (c == '{' || c == '}') && s.fl == flPowerShell:
			s.endWord()
			s.endSegment(false)
			s.i++
		default:
			s.wordChar()
		}
	}
	s.endWord()
	s.endSegment(false)
}

func (s *scanner) ampersand() {
	n := len(s.src)
	if s.fl == flBash && s.i+1 < n && s.src[s.i+1] == '>' {
		s.endWord()
		op := "&>"
		if s.i+2 < n && s.src[s.i+2] == '>' {
			op = "&>>"
		}
		s.i += len(op)
		s.pend = op
		return
	}
	s.endWord()
	if s.i+1 < n && s.src[s.i+1] == '&' {
		s.endSegment(false)
		s.i += 2
		return
	}
	// PowerShell's call operator: "& 'C:\x\tool.exe' args".
	if s.fl == flPowerShell && len(s.seg.words) == 0 && len(s.seg.redirs) == 0 {
		s.i++
		return
	}
	s.endSegment(false)
	s.i++
}

// redirect parses a redirection operator at s.i.
func (s *scanner) redirect() {
	n := len(s.src)
	c := s.src[s.i]
	if s.inWord && (allDigits(string(s.cur)) || (s.fl == flPowerShell && string(s.cur) == "*")) {
		s.cur, s.inWord = s.cur[:0], false // file descriptor prefix: 2> or *>
	} else {
		s.endWord()
	}
	has := func(k int, b byte) bool { return s.i+k < n && s.src[s.i+k] == b }
	if s.fl == flBash && has(1, '(') {
		s.group(s.i + 1) // process substitution <(...) or >(...)
		return
	}
	op := string(c)
	if c == '>' {
		switch {
		case has(1, '>'):
			op = ">>"
		case has(1, '|'):
			op = ">|"
		case has(1, '&'):
			op = ">&"
		}
	} else if s.fl == flBash {
		switch {
		case has(1, '<') && has(2, '<'):
			op = "<<<"
		case has(1, '<') && has(2, '-'):
			op = "<<-"
		case has(1, '<'):
			op = "<<"
		case has(1, '&'):
			op = "<&"
		}
	}
	s.i += len(op)
	s.pend = op
}

// group consumes a balanced "(...)" starting at index open (the "(") and adds
// the text to the current word and to the nested commands.
func (s *scanner) group(open int) {
	inner, end := matchParen(s.src, open, s.fl)
	s.seg.subs = append(s.seg.subs, inner)
	s.cur = append(s.cur, s.src[s.i:end]...)
	s.inWord = true
	s.i = end
}

func (s *scanner) wordChar() {
	n := len(s.src)
	c := s.src[s.i]
	switch {
	case c == '\\' && s.fl == flBash:
		s.backslash()
	case c == '`' && s.fl == flPowerShell, c == '^' && s.fl == flCmd:
		s.inWord = true
		if s.i+1 < n {
			if s.src[s.i+1] != '\n' {
				s.cur = append(s.cur, s.src[s.i+1])
			}
			s.i += 2
		} else {
			s.i++
		}
	case c == '$' && s.fl == flBash && s.i+1 < n && (s.src[s.i+1] == '\'' || s.src[s.i+1] == '"'):
		// $'...' (ANSI-C quoting) and $"..." (locale quoting): the $ is syntax.
		s.i++
		if s.src[s.i] == '\'' {
			s.ansiQuoted()
		} else {
			s.doubleQuoted()
		}
	case c == '\'' && s.fl != flCmd:
		s.singleQuoted()
	case c == '"':
		s.doubleQuoted()
	case c == '`' && s.fl == flBash:
		s.backtick()
	case c == '$' && s.i+1 < n && s.src[s.i+1] == '(' && s.fl != flCmd:
		s.group(s.i + 1)
		// group copied from s.i, which is the "$": keep it.
	case c == '$' && s.i+1 < n && s.src[s.i+1] == '{' && s.fl != flCmd:
		end := strings.IndexByte(s.src[s.i:], '}')
		if end < 0 {
			end = n - s.i - 1
		}
		s.cur = append(s.cur, s.src[s.i:s.i+end+1]...)
		s.inWord = true
		s.i += end + 1
	case c == '@' && s.fl == flPowerShell && s.i+1 < n && (s.src[s.i+1] == '"' || s.src[s.i+1] == '\'') && s.hereString():
	case c == '@' && s.fl == flPowerShell && s.i+1 < n && s.src[s.i+1] == '(':
		s.group(s.i + 1)
	default:
		s.inWord = true
		s.cur = append(s.cur, c)
		s.i++
	}
}

// backslash handles a POSIX escape. Backslashes inside what looks like a
// Windows path (C:\x, \\srv\share, .\x) are literal so such paths survive.
func (s *scanner) backslash() {
	n := len(s.src)
	cur := string(s.cur)
	if s.i+1 >= n {
		s.inWord = true
		s.cur = append(s.cur, '\\')
		s.i++
		return
	}
	next := s.src[s.i+1]
	if next == '\n' {
		s.i += 2
		return
	}
	winish := (len(cur) >= 2 && isAlpha(cur[0]) && cur[1] == ':') || strings.HasPrefix(cur, `\\`) ||
		cur == "." || cur == ".." || cur == "~" || (cur == "" && next == '\\' && s.i+2 < n && s.src[s.i+2] != ' ')
	s.inWord = true
	if winish {
		s.cur = append(s.cur, '\\')
		s.i++
		return
	}
	s.cur = append(s.cur, next)
	s.i += 2
}

func (s *scanner) singleQuoted() {
	n := len(s.src)
	s.inWord = true
	s.i++
	for s.i < n {
		c := s.src[s.i]
		if c == '\'' {
			if s.fl == flPowerShell && s.i+1 < n && s.src[s.i+1] == '\'' {
				s.cur = append(s.cur, '\'')
				s.i += 2
				continue
			}
			s.i++
			return
		}
		s.cur = append(s.cur, c)
		s.i++
	}
}

// ansiQuoted reads a $'...' string, decoding the common escapes so that
// $'r\x6d' is seen as "rm".
func (s *scanner) ansiQuoted() {
	n := len(s.src)
	s.inWord = true
	s.i++
	for s.i < n {
		c := s.src[s.i]
		switch {
		case c == '\'':
			s.i++
			return
		case c == '\\' && s.i+1 < n:
			s.i++
			switch e := s.src[s.i]; {
			case e == 'n':
				s.cur = append(s.cur, '\n')
			case e == 't':
				s.cur = append(s.cur, '\t')
			case e == 'x':
				v, used := hexByte(s.src[s.i+1:])
				if used > 0 {
					s.cur = append(s.cur, v)
					s.i += used
				}
			case e >= '0' && e <= '7':
				v, used := octByte(s.src[s.i:])
				s.cur = append(s.cur, v)
				s.i += used - 1
			default:
				s.cur = append(s.cur, e)
			}
			s.i++
		default:
			s.cur = append(s.cur, c)
			s.i++
		}
	}
}

func hexByte(s string) (byte, int) {
	var v byte
	used := 0
	for used < 2 && used < len(s) {
		d := hexVal(s[used])
		if d < 0 {
			break
		}
		v = v<<4 | byte(d)
		used++
	}
	return v, used
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func octByte(s string) (byte, int) {
	var v int
	used := 0
	for used < 3 && used < len(s) && s[used] >= '0' && s[used] <= '7' {
		v = v*8 + int(s[used]-'0')
		used++
	}
	return byte(v), used
}

func (s *scanner) doubleQuoted() {
	n := len(s.src)
	s.inWord = true
	s.i++
	for s.i < n {
		c := s.src[s.i]
		switch {
		case c == '"':
			if s.fl == flPowerShell && s.i+1 < n && s.src[s.i+1] == '"' {
				s.cur = append(s.cur, '"')
				s.i += 2
				continue
			}
			s.i++
			return
		case c == '\\' && s.fl == flBash && s.i+1 < n:
			nx := s.src[s.i+1]
			if nx == '$' || nx == '`' || nx == '"' || nx == '\\' {
				s.cur = append(s.cur, nx)
			} else if nx == '\n' {
				// line continuation
			} else {
				s.cur = append(s.cur, '\\', nx)
			}
			s.i += 2
		case c == '`' && s.fl == flPowerShell && s.i+1 < n:
			s.cur = append(s.cur, s.src[s.i+1])
			s.i += 2
		case c == '`' && s.fl == flBash:
			s.backtick()
		case c == '$' && s.i+1 < n && s.src[s.i+1] == '(' && s.fl != flCmd:
			inner, end := matchParen(s.src, s.i+1, s.fl)
			s.seg.subs = append(s.seg.subs, inner)
			s.cur = append(s.cur, s.src[s.i:end]...)
			s.i = end
		default:
			s.cur = append(s.cur, c)
			s.i++
		}
	}
}

func (s *scanner) backtick() {
	n := len(s.src)
	j := s.i + 1
	for j < n && s.src[j] != '`' {
		if s.src[j] == '\\' {
			j++
		}
		j++
	}
	if j > n {
		j = n
	}
	inner := s.src[s.i+1 : min(j, n)]
	s.seg.subs = append(s.seg.subs, inner)
	end := min(j+1, n)
	s.cur = append(s.cur, s.src[s.i:end]...)
	s.inWord = true
	s.i = end
}

// hereString consumes a PowerShell here-string (@" ... "@) as data and
// reports whether one started at s.i.
func (s *scanner) hereString() bool {
	q := s.src[s.i+1]
	rest := s.src[s.i+2:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 || strings.TrimSpace(rest[:nl]) != "" {
		return false
	}
	closer := "\n" + string(q) + "@"
	end := strings.Index(rest[nl:], closer)
	s.inWord = true
	if end < 0 {
		s.cur = append(s.cur, rest[nl+1:]...)
		s.i = len(s.src)
		return true
	}
	s.cur = append(s.cur, rest[nl+1:nl+end]...)
	s.i += 2 + nl + end + len(closer)
	return true
}

// endWord completes the word being built.
func (s *scanner) endWord() {
	if !s.inWord {
		return
	}
	w := string(s.cur)
	s.cur, s.inWord = s.cur[:0], false
	op := s.pend
	s.pend = ""
	switch op {
	case "":
		s.seg.words = append(s.seg.words, w)
	case "<<", "<<-":
		s.seg.hereDelim, s.seg.hereStrip = w, op == "<<-"
	case "<<<":
		// here-string: data
	case ">&", "<&":
		if allDigits(w) || w == "-" {
			return
		}
		s.seg.redirs = append(s.seg.redirs, Redirect{Op: op, Target: w})
	default:
		s.seg.redirs = append(s.seg.redirs, Redirect{Op: op, Target: w})
	}
}

// endSegment completes the current simple command.
func (s *scanner) endSegment(pipeNext bool) {
	if len(s.seg.words) > 0 || len(s.seg.redirs) > 0 || len(s.seg.subs) > 0 || s.seg.hereDelim != "" {
		s.seg.piped = s.piped
		if s.seg.hereDelim != "" {
			s.pendHere = append(s.pendHere, len(s.segs))
		}
		s.segs = append(s.segs, s.seg)
		s.seg = rawSegment{}
		s.piped = pipeNext
		return
	}
	s.piped = pipeNext
}

// consumeHeredocs reads the bodies of here-documents started on the line that
// just ended.
func (s *scanner) consumeHeredocs() {
	for _, idx := range s.pendHere {
		seg := &s.segs[idx]
		var body []string
		for s.i < len(s.src) {
			end := strings.IndexByte(s.src[s.i:], '\n')
			var line string
			if end < 0 {
				line, s.i = s.src[s.i:], len(s.src)
			} else {
				line, s.i = s.src[s.i:s.i+end], s.i+end+1
			}
			line = strings.TrimSuffix(line, "\r")
			cmp := line
			if seg.hereStrip {
				cmp = strings.TrimLeft(line, "\t")
			}
			if cmp == seg.hereDelim {
				break
			}
			body = append(body, line)
		}
		seg.heredoc = strings.Join(body, "\n")
	}
	s.pendHere = s.pendHere[:0]
}

// matchParen returns the text inside the balanced parentheses opening at
// src[open] and the index just after the closing one.
func matchParen(src string, open int, fl flavour) (inner string, end int) {
	depth := 0
	for j := open; j < len(src); j++ {
		switch c := src[j]; c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return src[open+1 : j], j + 1
			}
		case '\'':
			if fl != flCmd {
				if k := strings.IndexByte(src[j+1:], '\''); k >= 0 {
					j += k + 1
				}
			}
		case '"':
			for j++; j < len(src) && src[j] != '"'; j++ {
				if src[j] == '\\' && fl == flBash || src[j] == '`' && fl == flPowerShell {
					j++
				}
			}
		case '\\':
			if fl == flBash {
				j++
			}
		}
	}
	return src[min(open+1, len(src)):], len(src)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isAlpha(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
