package claudecode

import "strings"

const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// TypeMessage returns the bytes that type text into Claude Code's input box
// and submit it. Multi-line text is sent as a bracketed paste so its newlines
// are not taken for Enter; the closing marker is stripped from the text so it
// cannot end the paste early. The submitting Enter follows the paste.
func (Adapter) TypeMessage(text string) []byte {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if !strings.Contains(text, "\n") {
		return []byte(text + "\r")
	}
	for strings.Contains(text, pasteEnd) {
		text = strings.ReplaceAll(text, pasteEnd, "")
	}
	return []byte(pasteStart + text + pasteEnd + "\r")
}

// Interrupt returns Esc, which interrupts the current response or tool call
// and keeps the work done so far. (Esc does not produce a Stop hook.)
func (Adapter) Interrupt() []byte { return []byte("\x1b") }

// SideQuestion returns "/btw <text>" plus Enter, which asks a question that
// never enters the conversation history and works while Claude is busy. Its
// answer is drawn in Claude Code's own overlay inside the pane. Newlines are
// folded to spaces since /btw takes one line.
//
// UNVERIFIED (CC-08): /btw behaviour comes from the docs only.
func (Adapter) SideQuestion(text string) ([]byte, bool) {
	text = strings.Join(strings.Fields(text), " ")
	return []byte("/btw " + text + "\r"), true
}
