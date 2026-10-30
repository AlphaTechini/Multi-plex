package specview

import (
	"regexp"
	"strconv"
	"strings"
)

// blockKind is a markdown-lite block type.
type blockKind uint8

const (
	blkPara blockKind = iota
	blkHeading
	blkBullet
	blkOrdered
	blkTask
	blkQuote
	blkFence
	blkRule
	blkTable
	blkBlank
)

// block is one parsed block. List items carry their nesting level; task
// items carry the 1-based line of their checkbox in the source.
type block struct {
	kind   blockKind
	level  int      // heading level, or list nesting
	text   string   // paragraph, heading, item or quote text
	num    string   // ordered list number
	mark   byte     // task marker: ' ', 'x', '~'
	line   int      // source line (tasks)
	lines  []string // fence or table rows
	reqNum int      // requirement number in force (requirements doc), 0 none
}

var (
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	ruleRe     = regexp.MustCompile(`^\s{0,3}([-*_])(\s*[-*_]){2,}\s*$`)
	taskRe     = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX~\-])\]\*?\s*(.*?)\s*$`)
	bulletRe   = regexp.MustCompile(`^(\s*)[-*+]\s+(.*?)\s*$`)
	orderedRe  = regexp.MustCompile(`^(\s*)(\d+)[.)]\s+(.*?)\s*$`)
	quoteRe    = regexp.MustCompile(`^\s{0,3}>\s?(.*)$`)
	fenceRe    = regexp.MustCompile("^\\s{0,3}(```|~~~)")
	tableSepRe = regexp.MustCompile(`^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$`)
	reqHeadRe  = regexp.MustCompile(`^Requirement\s+(\d+)`)
	reqLineRe  = regexp.MustCompile(`^\s*[-*+]?\s*_?\*{0,2}Requirements?:?\*{0,2}.*$`)
	commentRe  = regexp.MustCompile(`<!--.*?-->`)
	taskNumRe  = regexp.MustCompile(`^(\d+(?:\.\d+)*)\.?\s+(.*)$`)
)

// parseBlocks splits markdown-lite source into blocks. A paragraph is its
// consecutive non-blank lines joined with spaces; a list item continues on
// the indented lines under it; "_Requirements: R1.1_" lines under a task are
// dropped because the task shows them as pills.
func parseBlocks(src string) []block {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = commentRe.ReplaceAllString(src, "")
	lines := strings.Split(src, "\n")
	var out []block
	var para []string
	reqNum := 0
	flushPara := func() {
		if len(para) > 0 {
			out = append(out, block{kind: blkPara, text: strings.Join(para, " "), reqNum: reqNum})
			para = nil
		}
	}
	lastItem := func() *block {
		if n := len(out); n > 0 && (out[n-1].kind == blkBullet || out[n-1].kind == blkOrdered || out[n-1].kind == blkTask) && len(para) == 0 {
			return &out[n-1]
		}
		return nil
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		if m := fenceRe.FindStringSubmatch(line); m != nil {
			flushPara()
			var body []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), m[1]); i++ {
				body = append(body, strings.TrimRight(lines[i], " \t\r"))
			}
			out = append(out, block{kind: blkFence, lines: body})
			continue
		}
		if strings.TrimSpace(line) == "" {
			flushPara()
			if n := len(out); n > 0 && out[n-1].kind != blkBlank {
				out = append(out, block{kind: blkBlank})
			}
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			flushPara()
			if r := reqHeadRe.FindStringSubmatch(strings.TrimSpace(m[2])); r != nil {
				reqNum, _ = strconv.Atoi(r[1])
			}
			out = append(out, block{kind: blkHeading, level: len(m[1]), text: m[2]})
			continue
		}
		if ruleRe.MatchString(line) {
			flushPara()
			out = append(out, block{kind: blkRule})
			continue
		}
		if strings.Contains(line, "|") && i+1 < len(lines) && tableSepRe.MatchString(lines[i+1]) && strings.Contains(lines[i+1], "-") {
			flushPara()
			rows := []string{line}
			for i += 2; i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != ""; i++ {
				rows = append(rows, lines[i])
			}
			i--
			out = append(out, block{kind: blkTable, lines: rows})
			continue
		}
		if m := quoteRe.FindStringSubmatch(line); m != nil {
			flushPara()
			if n := len(out); n > 0 && out[n-1].kind == blkQuote {
				out[n-1].text += " " + strings.TrimSpace(m[1])
			} else {
				out = append(out, block{kind: blkQuote, text: strings.TrimSpace(m[1])})
			}
			continue
		}
		if m := taskRe.FindStringSubmatch(line); m != nil {
			flushPara()
			mark := byte(' ')
			switch m[2] {
			case "x", "X":
				mark = 'x'
			case "~", "-":
				mark = '~'
			}
			out = append(out, block{kind: blkTask, level: indentLevel(m[1]), text: m[3], mark: mark, line: i + 1})
			continue
		}
		if m := orderedRe.FindStringSubmatch(line); m != nil {
			flushPara()
			out = append(out, block{kind: blkOrdered, level: indentLevel(m[1]), num: m[2], text: m[3], reqNum: reqNum})
			continue
		}
		if m := bulletRe.FindStringSubmatch(line); m != nil {
			flushPara()
			if reqLineRe.MatchString(line) && inTask(out) {
				continue // shown as pills on the task
			}
			out = append(out, block{kind: blkBullet, level: indentLevel(m[1]), text: m[2]})
			continue
		}
		if it := lastItem(); it != nil && leadingSpace(line) > 0 {
			if reqLineRe.MatchString(line) && it.kind == blkTask {
				continue
			}
			it.text += " " + strings.TrimSpace(line)
			continue
		}
		para = append(para, strings.TrimSpace(line))
	}
	flushPara()
	for len(out) > 0 && out[len(out)-1].kind == blkBlank {
		out = out[:len(out)-1]
	}
	return out
}

// inTask reports whether the last block is a task or one of its children.
func inTask(out []block) bool {
	for i := len(out) - 1; i >= 0; i-- {
		switch out[i].kind {
		case blkTask:
			return true
		case blkBullet:
			continue
		default:
			return false
		}
	}
	return false
}

// indentLevel converts leading whitespace to a nesting level (2 cells each).
func indentLevel(ws string) int {
	n := 0
	for _, c := range ws {
		if c == '\t' {
			n += 2
		} else {
			n++
		}
	}
	return n / 2
}

func leadingSpace(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }
